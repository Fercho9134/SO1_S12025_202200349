package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
	"net"
	"os"
	"sync"
	"servidor-api-go/internal/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type grpcServer struct {
	proto.UnimplementedWeatherServiceServer
}

func (s *grpcServer) PublishToRabbitMQ(ctx context.Context, tweet *proto.WeatherRequest) (*proto.WeatherResponse, error) {
	log.Printf("Received tweet for RabbitMQ: %v", tweet)
	return &proto.WeatherResponse{Success: true, Message: "Forwarded to RabbitMQ writer"}, nil
}

func (s *grpcServer) PublishToKafka(ctx context.Context, tweet *proto.WeatherRequest) (*proto.WeatherResponse, error) {
	log.Printf("Received tweet for Kafka: %v", tweet)
	return &proto.WeatherResponse{Success: true, Message: "Forwarded to Kafka writer"}, nil
}

func startGRPCServer() {
	lis, err := net.Listen("tcp", ":50050")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	proto.RegisterWeatherServiceServer(s, &grpcServer{})

	log.Printf("gRPC server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func main() {
	// Start gRPC server in a goroutine
	go startGRPCServer()

	// HTTP Server setup
	kafkaAddr := getEnv("KAFKA_WRITER_ADDR", "go-kafka-writer:50051")
	rabbitAddr := getEnv("RABBITMQ_WRITER_ADDR", "go-rabbitmq-writer:50052")

	kafkaConn := setupGRPCConn(kafkaAddr)
	defer kafkaConn.Close()

	rabbitConn := setupGRPCConn(rabbitAddr)
	defer rabbitConn.Close()

	http.HandleFunc("/input", handleInput(kafkaConn, rabbitConn))
	http.HandleFunc("/health", handleHealthCheck)

	log.Printf("HTTP server running on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func setupGRPCConn(addr string) *grpc.ClientConn {
	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
		grpc.WithTimeout(5*time.Second))
	if err != nil {
		log.Fatalf("Failed to connect to %s: %v", addr, err)
	}
	return conn
}

func handleInput(kafkaConn, rabbitConn *grpc.ClientConn) http.HandlerFunc {
	clientKafka := proto.NewWeatherServiceClient(kafkaConn)
	clientRabbit := proto.NewWeatherServiceClient(rabbitConn)

	

	return func(w http.ResponseWriter, r *http.Request) {

		log.Printf("--> Received HTTP request on %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)

		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var tweet proto.WeatherRequest
		if err := json.NewDecoder(r.Body).Decode(&tweet); err != nil {
			log.Printf("Invalid request body: %v", err)
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		log.Printf("Processing tweet: %+v", tweet)

		var wg sync.WaitGroup
		wg.Add(2)
		errChan := make(chan error, 2)

		go func() {
			defer wg.Done()
			_, err := clientKafka.PublishToKafka(r.Context(), &tweet)
			if err != nil {
				errChan <- err
				log.Printf("Kafka publish error: %v", err)
			}else {
				log.Printf("Kafka publish success: %v", tweet)
			}
		}()

		go func() {
			defer wg.Done()
			_, err := clientRabbit.PublishToRabbitMQ(r.Context(), &tweet)
			if err != nil {
				errChan <- err
				log.Printf("RabbitMQ publish error: %v", err)
			}else {
				log.Printf("RabbitMQ publish success: %v", tweet)
			}
		}()

		wg.Wait()
		close(errChan)

		if len(errChan) > 0 {
			http.Error(w, "Failed to process some messages", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]string{"status": "accepted"})
	}
}

func handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func getEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}