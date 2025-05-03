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

	amqp "github.com/rabbitmq/amqp091-go" // Librería oficial de RabbitMQ
	"github.com/go-redis/redis/v8"      // Librería para Redis/Valkey
	
)

// WeatherMessage estructura para decodificar el mensaje JSON de RabbitMQ
type WeatherMessage struct {
	Description string `json:"description"`
	Country     string `json:"country"`
	Weather     string `json:"weather"`
}

const (
	valkeyAddrEnv  = "VALKEY_ADDR"  // Variable de entorno para la dirección de Valkey
	rabbitMQURLEnv = "RABBITMQ_URL" // Variable de entorno para la URL de RabbitMQ
	rabbitMQQueue  = "weather-tweets" // Cola a consumir (debe coincidir con el publicador)
	valkeyCountryHash = "country_counts" // Nombre de la tabla hash en Valkey para contadores por país
	valkeyTotalKey   = "total_messages" // Clave para el contador total en Valkey
	batchSize        = 50 // Procesamiento por lotes para Valkey (a nivel del consumidor)
	healthPort       = "8080"
)

var (
	ctx = context.Background() // Contexto para operaciones de Valkey
	processedCount int64
	errorCount     int64
	processingMutex sync.Mutex // Mutex para proteger los contadores globales
)

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func main() {
	// Conexión a RabbitMQ
	rabbitMQURL := getEnv(rabbitMQURLEnv, "amqp://rabbitmq:5672")
	conn, err := amqp.Dial(rabbitMQURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ at %s: %v", rabbitMQURL, err)
	}
	defer conn.Close()
	log.Printf("RabbitMQ Consumer connected to RabbitMQ at %s", rabbitMQURL)

	// Abrir canal RabbitMQ
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("Failed to open a RabbitMQ channel: %v", err)
	}
	defer ch.Close()
	log.Println("RabbitMQ channel opened successfully")

	// Declarar la cola (asegurarse de que exista y sea durable, debe coincidir con el publicador)
	// Declarar la cola aquí es idempotente; no pasa nada si ya existe.
	q, err := ch.QueueDeclare(
		rabbitMQQueue, 
		true,          
		false,         
		false,         
		false,         
		nil,           
	)
	if err != nil {
		log.Fatalf("Failed to declare RabbitMQ queue '%s': %v", rabbitMQQueue, err)
	}
	log.Printf("RabbitMQ Consumer ensuring queue '%s' exists (%d messages, %d consumers)",
		q.Name, q.Messages, q.Consumers)

	// Registrar consumidor en la cola
	
	messages, err := ch.Consume(
		q.Name,               // queue
		"",                   // consumer name
		false,                // auto-ack 
		false,                // exclusive
		false,                // no-local
		false,                // no-wait
		nil,                  // args
	)
	if err != nil {
		log.Fatalf("Failed to register a RabbitMQ consumer: %v", err)
	}
	log.Printf("RabbitMQ Consumer registered on queue '%s'. Waiting for deliveries...", q.Name)

	// Conexión a Valkey (misma librería de Redis)
	valkeyAddr := getEnv(valkeyAddrEnv, "valkey:6379") // Usar el nombre del Service de Valkey
	valkeyClient := redis.NewClient(&redis.Options{
		Addr: valkeyAddr,
		MinIdleConns: 5,
		PoolSize:     20,
		PoolTimeout:  30 * time.Second,
	})
	defer valkeyClient.Close()

	// Verificar conexión a Valkey
	_, err = valkeyClient.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("Failed to connect to Valkey at %s: %v", valkeyAddr, err)
	}
	log.Printf("RabbitMQ Consumer connected to Valkey at %s", valkeyAddr)

	// Health Check en una goroutine separada
	go func() {
		http.HandleFunc("/health", healthHandler(valkeyClient, conn)) // Pasar cliente Valkey y conexión AMQP
		log.Printf("Health check server running on :%s", healthPort)
		if err := http.ListenAndServe(":"+healthPort, nil); err != nil {
			log.Printf("Health check server error: %v", err)
		}
	}()


	// Goroutine para manejar señales de apagado (CTRL+C, etc.)
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)

	// Canal para deliveries batch
	deliveryBatch := make(chan amqp.Delivery, batchSize*2)
	var wg sync.WaitGroup

	// Iniciar workers para procesamiento batch
	numWorkers := 5 
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go processBatchWorker(valkeyClient, deliveryBatch, &wg)
	}

	log.Println("Starting RabbitMQ consumer loop, forwarding deliveries to workers...")

	for {
		select {
		case sig := <-sigchan:
			log.Printf("Caught signal %v: initiating shutdown", sig)
			// Cerrar el canal de batching para indicar a los workers que no habrá más mensajes
			close(deliveryBatch)
			log.Println("RabbitMQ delivery channel closed. Waiting for workers to finish...")
			wg.Wait() // Esperar a que todas las goroutines de procesamiento terminen
			log.Println("Shutdown complete. Total processed: %d, errors: %d", processedCount, errorCount)
			// Cerrar conexiones (defer conn.Close() y defer ch.Close() se encargarán)
			return // Salir de main
		case d, ok := <-messages: // Leer del canal de Deliveries de RabbitMQ
			if !ok {
				// El canal de mensajes de RabbitMQ se cerró (ej: conexión perdida, canal cerrado)
				log.Println("RabbitMQ messages channel closed. Terminating.")
				// Cerrar el canal de batching para terminar workers
				close(deliveryBatch)
				log.Println("RabbitMQ delivery channel closed. Waiting for workers to finish...")
				wg.Wait()
				return // Salir de main
			}

			
			log.Printf("Received RabbitMQ delivery tag: %d", d.DeliveryTag)
			incrementProcessedCount() // Incrementamos el contador aquí (solo si se recibe)

			// Enviar la delivery al canal de batching para ser procesada por un worker
			deliveryBatch <- d

			// No hacer Ack/Nack aquí. Esto se hace en el worker *después* de escribir en DB.
		}
	}
}


// processBatchWorker lee deliveries del canal y las procesa en lotes
func processBatchWorker(valkeyClient *redis.Client, batchChan <-chan amqp.Delivery, wg *sync.WaitGroup) {
	defer wg.Done()

	var batch []amqp.Delivery
	ticker := time.NewTicker(1 * time.Second) // Ticker para forzar el procesamiento de lotes incompletos
	defer ticker.Stop()

	for {
		select {
		case delivery, ok := <-batchChan:
			if !ok {
				
				if len(batch) > 0 {
					// Procesar el lote final
					processDeliveriesBatch(valkeyClient, batch)
				}
				log.Println("Batch worker channel closed. Exiting.")
				return // Salir de la goroutine del worker
			}
			batch = append(batch, delivery)
			if len(batch) >= batchSize {
				// Procesar lote completo
				processDeliveriesBatch(valkeyClient, batch)
				batch = nil // Reiniciar el lote
			}
		case <-ticker.C:
			// Ticker disparado, procesar lote actual si no está vacío
			if len(batch) > 0 {
				processDeliveriesBatch(valkeyClient, batch)
				batch = nil // Reiniciar el lote
			}
		}
	}
}

// processDeliveriesBatch procesa un lote de deliveries y actualiza Valkey.
// NO CONFIRMA (Ack) ni rechaza (Nack) las deliveries aquí.
// La confirmación/rechazo ocurre en el worker que llamó a esta función,
// *después* de que esta función retorne.
func processDeliveriesBatch(valkeyClient *redis.Client, deliveries []amqp.Delivery) {
	if len(deliveries) == 0 {
		return // No hay deliveries para procesar
	}

	pipe := valkeyClient.Pipeline()
	counts := make(map[string]int64)

	// Procesar cada delivery en el lote
	for _, d := range deliveries {
		var weatherMsg WeatherMessage
		// Usamos d.Body porque es el contenido del mensaje de RabbitMQ
		if err := json.Unmarshal(d.Body, &weatherMsg); err != nil {
			log.Printf("Failed to unmarshal delivery body tag %d: %v", d.DeliveryTag, err)
			incrementErrorCount()
			continue 
		}

		country := weatherMsg.Country
		if country == "" {
			country = "UNKNOWN"
		}
		counts[country]++

		log.Printf("Processing delivery tag %d: %s (%s)", d.DeliveryTag, weatherMsg.Description, country)
		
	}

	if len(counts) == 0 {
		log.Printf("Batch of %d deliveries contained no processable messages.", len(deliveries))
        
		return
	}


	// --- Actualizar contadores en Valkey ---
	for country, count := range counts {
		pipe.HIncrBy(ctx, valkeyCountryHash, country, count) // Incrementar contador por país
	}
	pipe.IncrBy(ctx, valkeyTotalKey, int64(len(deliveries))) // Incrementar contador total del lote

	if _, err := pipe.Exec(ctx); err != nil {
		log.Printf("Failed to update Valkey with batch of %d deliveries: %v", len(deliveries), err)
		incrementErrorCount()
		
		return
	}
	// ------------------------------------

	// --- Éxito en el procesamiento y escritura en Valkey ---
	log.Printf("Successfully processed batch of %d deliveries and wrote to Valkey", len(deliveries))

	// NOTA: La CONFIRMACIÓN (Ack) a RabbitMQ NO SE HACE AQUÍ.
	// Se hace en la función worker llamadora *después* de que esta función retorne sin error.
}

// healthHandler verifica la conexión a Valkey y a RabbitMQ
func healthHandler(valkeyClient *redis.Client, rabbitMQConn *amqp.Connection) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Verificar conexión a Valkey
		if _, err := valkeyClient.Ping(ctx).Result(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "Valkey connection error: %v\n", err)
			return
		}

		// Verificar conexión a RabbitMQ
		// amqp.Connection.IsClosed() verifica si la conexión *ya* está cerrada.
		// Un ping o un intento de abrir un canal es más robusto para verificar si está *activa*.
		// Un intento rápido de abrir/cerrar un canal puede servir como "ping".
		if rabbitMQConn == nil || rabbitMQConn.IsClosed() {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, "RabbitMQ connection is closed or nil")
			return
		}
		// Intento simple de verificar si el canal está activo
		ch, err := rabbitMQConn.Channel()
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, "RabbitMQ channel check failed: %v\n", err)
			return
		}
		ch.Close() // Cerrar el canal inmediatamente

		processingMutex.Lock()
		defer processingMutex.Unlock()
		fmt.Fprintf(w, "OK\nValkey: Connected\nRabbitMQ: Connected\nProcessed Deliveries: %d\nErrors: %d", processedCount, errorCount)
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