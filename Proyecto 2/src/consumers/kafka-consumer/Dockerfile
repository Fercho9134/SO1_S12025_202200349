# Etapa de construcción usando Debian (basado en glibc)
FROM golang:1.24-bullseye as builder

WORKDIR /app

# Instalar dependencias nativas necesarias para confluent-kafka-go
RUN apt-get update && apt-get install -y librdkafka-dev

# Copiar los archivos de módulos y descargar dependencias
COPY go.mod go.sum ./
RUN go mod download

# Copiar el código fuente
COPY . .

# Habilitar CGO y compilar para Linux
ENV CGO_ENABLED=1 GOOS=linux
RUN go build -o kafka-consumer .

# Imagen final
FROM debian:bullseye-slim

WORKDIR /app

# Copiar el binario desde la etapa de construcción
COPY --from=builder /app/kafka-consumer .

EXPOSE 8080
ENTRYPOINT ["/app/kafka-consumer"]
