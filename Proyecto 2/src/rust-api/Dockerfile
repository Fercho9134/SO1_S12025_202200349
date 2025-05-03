# Usa Rust 1.81 o superior
FROM rust:1.81-slim-bookworm as builder

# Instala dependencias del sistema
RUN apt-get update && apt-get install -y \
    pkg-config \
    libssl-dev \
    protobuf-compiler \
    && rm -rf /var/lib/apt/lists/*

# Configura el workspace
WORKDIR /app
COPY . .

# Compila la aplicación
RUN cargo build --release

# --- Imagen final ---
FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y \
    libssl3 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /app/target/release/rust-api .
COPY --from=builder /app/config /app/config

EXPOSE 8080
CMD ["./rust-api"]