package main

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"os"
	"time"
	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
	"servidor-api-go/internal/proto" // Ajusta la ruta a tu módulo
)

// Server implementa el servicio gRPC
type rabbitMQServer struct {
	proto.UnimplementedWeatherServiceServer
	conn *amqp.Connection
}

func (s *rabbitMQServer) PublishToRabbitMQ(ctx context.Context, tweet *proto.WeatherRequest) (*proto.WeatherResponse, error) {
	
	log.Printf("Received gRPC call PublishToRabbitMQ with tweet: %+v", tweet)
	
	ch, err := s.conn.Channel()
	
	if err != nil {
		log.Printf("Failed to open RabbitMQ channel: %v", err)
		return &proto.WeatherResponse{
			Success: false,
			Message: "Failed to open channel",
		}, err
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(
		"weather-tweets", // name
		true,             // durable
		false,            // delete when unused
		false,            // exclusive
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		log.Printf("Failed to declare queue: %v", err)
		return &proto.WeatherResponse{
			Success: false,
			Message: "Failed to declare queue",
		}, err
	}

	message := map[string]string{
		"description": tweet.GetDescription(),
		"country":     tweet.GetCountry(),
		"weather":     tweet.GetWeather(),
	}

	body, err := json.Marshal(message)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return &proto.WeatherResponse{
			Success: false,
			Message: "Failed to marshal message",
		}, err
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	log.Printf("Attempting to publish message to RabbitMQ queue %s", q.Name)

	err = ch.PublishWithContext(ctx,
		"",     // exchange
		q.Name, // routing key
		false,  // mandatory
		false,  // immediate
		amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			ContentType: "application/json",
			Body:        body,
		},
	)

	if err != nil {
		log.Printf("Failed to publish message to RabbitMQ: %v", err)
		return &proto.WeatherResponse{
			Success: false,
			Message: "Failed to publish message",
		}, err
	}

	log.Printf("Message successfully published to RabbitMQ queue %s", q.Name)

	return &proto.WeatherResponse{
		Success: true,
		Message: "Message published to RabbitMQ",
	}, nil
}

func (s *rabbitMQServer) PublishToKafka(ctx context.Context, tweet *proto.WeatherRequest) (*proto.WeatherResponse, error) {
	return &proto.WeatherResponse{
		Success: false,
		Message: "This service only handles RabbitMQ messages",
	}, nil
}

func main() {
	// RabbitMQ Connection
	conn, err := amqp.Dial(getEnv("RABBITMQ_URL", "amqp://rabbitmq:5672"))
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	log.Println("Connected to RabbitMQ")

	// gRPC Server
	lis, err := net.Listen("tcp", ":50052")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	s := grpc.NewServer()
	proto.RegisterWeatherServiceServer(s, &rabbitMQServer{conn: conn})

	log.Println("RabbitMQ Writer gRPC server listening on :50052")
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