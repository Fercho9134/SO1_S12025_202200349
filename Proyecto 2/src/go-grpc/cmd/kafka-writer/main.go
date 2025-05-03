package main

import (
	"context"
	"log"
	"net"
	"os"
	"encoding/json"
	"github.com/confluentinc/confluent-kafka-go/kafka"
	"google.golang.org/grpc"
	"servidor-api-go/internal/proto"
)

// Server implementa el servicio gRPC
type kafkaServer struct {
	proto.UnimplementedWeatherServiceServer
	producer *kafka.Producer
}

func (s *kafkaServer) PublishToKafka(ctx context.Context, tweet *proto.WeatherRequest) (*proto.WeatherResponse, error) {
	
	log.Printf("Received gRPC call PublishToKafka with tweet: %+v", tweet)
	message := map[string]string{
		"description": tweet.GetDescription(),
		"country":     tweet.GetCountry(),
		"weather":     tweet.GetWeather(),
	}

	jsonData, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return &proto.WeatherResponse{
			Success: false,
			Message: "Failed to marshal message",
		}, err
	}

	topic := "weather-tweets"
	deliveryChan := make(chan kafka.Event)
	defer close(deliveryChan)

	log.Printf("Attempting to produce message to topic %s", topic)

	err = s.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          jsonData,
	}, deliveryChan)

	if err != nil {
		log.Printf("Failed to produce message to Kafka: %v", err)
		return &proto.WeatherResponse{
			Success: false,
			Message: "Failed to produce message",
		}, err
	}

	e := <-deliveryChan
	m := e.(*kafka.Message)

	if m.TopicPartition.Error != nil {
		log.Printf("Delivery failed: %v", m.TopicPartition.Error)
		return &proto.WeatherResponse{
			Success: false,
			Message: "Delivery failed",
		}, m.TopicPartition.Error
	}

	log.Printf("Message successfully published to Kafka topic %s [%d] at offset %v",
        *m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset)

	return &proto.WeatherResponse{
		Success: true,
		Message: "Message published to Kafka",
	}, nil
}

func (s *kafkaServer) PublishToRabbitMQ(ctx context.Context, tweet *proto.WeatherRequest) (*proto.WeatherResponse, error) {
	return &proto.WeatherResponse{
		Success: false,
		Message: "This service only handles Kafka messages",
	}, nil
}

func main() {
	// Kafka Producer Configuration
	config := &kafka.ConfigMap{
		"bootstrap.servers":  getEnv("KAFKA_BOOTSTRAP_SERVERS", "kafka:9092"),
		"client.id":          "kafka-writer",
		"acks":               "all",
		"message.timeout.ms": 3000,
	}

	producer, err := kafka.NewProducer(config)
	if err != nil {
		log.Fatalf("Failed to create Kafka producer: %v", err)
	}
	defer producer.Close()

	// gRPC Server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	proto.RegisterWeatherServiceServer(s, &kafkaServer{producer: producer})

	log.Println("Kafka Writer gRPC server listening on :50051 :)")
	if err := s.Serve(lis); err != nil {
		log.Fatalf("Failed to serve: %v", err)
	}
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}