package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/go-redis/redis/v8"
)

// WeatherMessage estructura para decodificar el mensaje JSON de Kafka
type WeatherMessage struct {
	Description string `json:"description"`
	Country     string `json:"country"`
	Weather     string `json:"weather"`
}

const (
	redisAddrEnv     = "REDIS_ADDR"
	kafkaBrokerEnv   = "KAFKA_BOOTSTRAP_SERVERS"
	kafkaTopic       = "weather-tweets"
	kafkaGroupIDEnv  = "KAFKA_CONSUMER_GROUP_ID"
	redisCountryHash = "country_counts"
	redisTotalKey    = "total_messages"
	batchSize        = 50 // Procesamiento por lotes para Redis
	healthPort       = "8080"
)

var (
	ctx             = context.Background()
	processedCount  int64
	errorCount      int64
	processingMutex sync.Mutex
)

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func main() {
	// Configuración mejorada del consumidor de Kafka
	kafkaBrokers := getEnv(kafkaBrokerEnv, "kafka-service:9092")
	kafkaGroupID := getEnv(kafkaGroupIDEnv, "weather-consumer-group")

	consumerConfig := &kafka.ConfigMap{
		"bootstrap.servers":        kafkaBrokers,
		"group.id":                 kafkaGroupID,
		"auto.offset.reset":        "earliest",
		"enable.auto.commit":       false,
		"fetch.max.bytes":          1048576,
		"max.partition.fetch.bytes": 1048576,
		"session.timeout.ms":       60000,
		"heartbeat.interval.ms":    20000,
	}

	consumer, err := kafka.NewConsumer(consumerConfig)

	if err != nil {
		log.Fatalf("Failed to create Kafka consumer: %v", err)
	}
	defer consumer.Close()

	// Suscripción al topic
	err = consumer.SubscribeTopics([]string{kafkaTopic}, nil)
	if err != nil {
		log.Fatalf("Failed to subscribe to topic %s: %v", kafkaTopic, err)
	}

	// Conexión a Redis con configuración mejorada
	redisAddr := getEnv(redisAddrEnv, "redis-service:6379")
	redisClient := redis.NewClient(&redis.Options{
		Addr:         redisAddr,
		MinIdleConns: 5,
		PoolSize:     20,
		PoolTimeout:  30 * time.Second,
	})
	defer redisClient.Close()

	// Health Check en una goroutine separada
	go func() {
		http.HandleFunc("/health", healthHandler(redisClient, consumer))
		log.Printf("Health check server running on :%s", healthPort)
		if err := http.ListenAndServe(":"+healthPort, nil); err != nil {
			log.Printf("Health check server error: %v", err)
		}
	}()

	// Manejo de señales para shutdown graceful
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	// Canal para mensajes batch
	messageBatch := make(chan *kafka.Message, batchSize*2)
	var wg sync.WaitGroup

	// Iniciar workers para procesamiento batch
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go processBatchWorker(redisClient, messageBatch, consumer, &wg)
	}

	log.Println("Starting Kafka consumer loop...")
	run := true
	for run {
		select {
		case sig := <-sigchan:
			log.Printf("Caught signal %v: terminating", sig)
			run = false
		default:
			msg, err := consumer.ReadMessage(100 * time.Millisecond)
			if err != nil {
				if err.(kafka.Error).Code() != kafka.ErrTimedOut {
					log.Printf("Consumer error: %v", err)
					incrementErrorCount()
				}
				continue
			}

			messageBatch <- msg
			incrementProcessedCount()
		}
	}

	close(messageBatch)
	wg.Wait()

	// Commit final de offsets pendientes
	if _, err := consumer.Commit(); err != nil {
		log.Printf("Final commit failed: %v", err)
	}

	log.Printf("Shutdown complete. Total processed: %d, errors: %d", processedCount, errorCount)
}

func processBatchWorker(redisClient *redis.Client, batchChan <-chan *kafka.Message, consumer *kafka.Consumer, wg *sync.WaitGroup) {
	defer wg.Done()

	var batch []*kafka.Message
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg, ok := <-batchChan:
			if !ok {
				if len(batch) > 0 {
					processMessagesBatch(redisClient, batch, consumer)
				}
				return
			}
			batch = append(batch, msg)
			if len(batch) >= batchSize {
				processMessagesBatch(redisClient, batch, consumer)
				batch = nil
			}
		case <-ticker.C:
			if len(batch) > 0 {
				processMessagesBatch(redisClient, batch, consumer)
				batch = nil
			}
		}
	}
}

func processMessagesBatch(redisClient *redis.Client, messages []*kafka.Message, consumer *kafka.Consumer) { // <-- Pasar el consumidor como argumento
	if len(messages) == 0 {
		return // No hay mensajes para procesar
	}

	pipe := redisClient.Pipeline()
	counts := make(map[string]int64)

    // Mapa para rastrear el offset más alto por partición en este lote
    latestOffsets := make(map[int32]kafka.Offset)

	for _, msg := range messages {
        // Actualizar el offset más alto visto para esta partición
        // El offset a confirmar para una partición es el offset del último mensaje + 1
        if msg.TopicPartition.Offset > latestOffsets[msg.TopicPartition.Partition] {
            latestOffsets[msg.TopicPartition.Partition] = msg.TopicPartition.Offset
        }


		var weatherMsg WeatherMessage
		if err := json.Unmarshal(msg.Value, &weatherMsg); err != nil {
			log.Printf("Failed to unmarshal message at offset %v: %v", msg.TopicPartition.Offset, err)
			incrementErrorCount()
			continue
		}

		//Imprimir el mensaje procesado
		log.Printf("Processed message: %+v", weatherMsg)

		country := weatherMsg.Country
		if country == "" {
			country = "UNKNOWN"
		}
		counts[country]++
	}

	// Actualizar Redis
	for country, count := range counts {
		pipe.HIncrBy(ctx, redisCountryHash, country, count)
	}
	pipe.IncrBy(ctx, redisTotalKey, int64(len(messages)))

	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("Failed to update Redis: %v", err)
		incrementErrorCount()
		return
	}

    // Crear una lista de TopicPartitions para confirmar
    var committedOffsets []kafka.TopicPartition
    for partition, offset := range latestOffsets {
        // Confirmar el offset del último mensaje + 1 para cada partición
        committedOffsets = append(committedOffsets, kafka.TopicPartition{
            Topic:     messages[0].TopicPartition.Topic, // Asumiendo un solo topic
            Partition: partition,
            Offset:    offset + 1, 
        })
    }

    // Ejecutar el commit
    _, err := consumer.CommitOffsets(committedOffsets) // Commit the batch offsets
    if err != nil {
        log.Printf("Failed to commit offsets for batch: %v", err)
        incrementErrorCount() 
        
    } else {
        // --- Log de commit exitoso del lote ---
        log.Printf("Successfully processed and committed batch of %d messages up to offsets: %+v", len(messages), committedOffsets)
        // ------------------------------------
    }
    // ----------------------------------------------------------

}

func healthHandler(redisClient *redis.Client, consumer *kafka.Consumer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verificar conexión a Redis
		if _, err := redisClient.Ping(ctx).Result(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Redis connection error: %v", err)
			return
		}

		// Verificar conexión a Kafka
		if _, err := consumer.GetMetadata(nil, true, 5000); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Kafka connection error: %v", err)
			return
		}

		processingMutex.Lock()
		defer processingMutex.Unlock()
		fmt.Fprintf(w, "OK\nProcessed: %d\nErrors: %d", processedCount, errorCount)
	}
}

func incrementProcessedCount() {
	processingMutex.Lock()
	defer processingMutex.Unlock()
	processedCount++
}

func incrementErrorCount() {
	processingMutex.Lock()
	defer processingMutex.Unlock()
	errorCount++
}