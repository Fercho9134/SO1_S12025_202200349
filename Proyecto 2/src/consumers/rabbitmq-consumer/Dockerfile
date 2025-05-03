# Etapa de construcción usando Debian
FROM golang:1.24-bullseye as builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download


COPY . .

ENV CGO_ENABLED=0 GOOS=linux
RUN go build -o rabbitmq-consumer .


FROM debian:bullseye-slim

WORKDIR /app

COPY --from=builder /app/rabbitmq-consumer .

# Exponer el puerto del Health Check
EXPOSE 8080

# Comando para ejecutar la aplicación
ENTRYPOINT ["/app/rabbitmq-consumer"]