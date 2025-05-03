#!/bin/bash

# Función para generar nombres únicos
generar_nombre() {
    echo "contenedor_$(date +%s%N | sha256sum | base64 | head -c 8)"
}

# Limitar recursos
LIMITE_CPU="1"  # 1 núcleo de CPU
LIMITE_RAM="512m" # 512 MB de RAM

# Nota: El contenedor de disco se ha omitido debido a problemas con el hardware.

# Crear 3 contenedores específicos (CPU, RAM, I/O)
echo "Creando contenedores específicos..."
docker run -d --name "cpu_$(generar_nombre)" --cpus="$LIMITE_CPU" --cgroupns=host alpine-stress stress --cpu 1
docker run -d --name "ram_$(generar_nombre)" --memory="$LIMITE_RAM" --cgroupns=host alpine-stress stress --vm 1 --vm-bytes 256M
docker run -d --name "io_$(generar_nombre)" --cgroupns=host alpine-stress stress --io 1
#docker run -d --name "disco_$(generar_nombre)" \
#    --device-read-bps $DISPOSITIVO_DISCO:$LIMITE_IO_READ \
#    --device-write-bps $DISPOSITIVO_DISCO:$LIMITE_IO_WRITE \
#    --device-read-iops $DISPOSITIVO_DISCO:$LIMITE_IOPS_READ \
#    --device-write-iops $DISPOSITIVO_DISCO:$LIMITE_IOPS_WRITE \
#    --cgroupns=host alpine-stress stress --hdd 1


# Crear 7 contenedores aleatorios (CPU, RAM, I/O)
TIPOS=("cpu" "ram" "io")  # Se ha omitido "disco"
for i in {1..2}; do
    TIPO=${TIPOS[$RANDOM % ${#TIPOS[@]}]}
    echo "Creando contenedor aleatorio de tipo $TIPO..."
    case $TIPO in
        "cpu")
            docker run -d --name "cpu_$(generar_nombre)" --cpus="$LIMITE_CPU" --cgroupns=host alpine-stress stress --cpu 1
            ;;
        "ram")
            docker run -d --name "ram_$(generar_nombre)" --memory="$LIMITE_RAM" --cgroupns=host alpine-stress stress --vm 1 --vm-bytes 256M
            ;;
        "io")
            docker run -d --name "io_$(generar_nombre)" --cgroupns=host alpine-stress stress --io 1
            ;;
        # Nota: El caso "disco" se ha omitido debido a problemas con el hardware.
        # "disco")
        #     docker run -d --name "disco_$(generar_nombre)" \
        #         --device-read-bps $DISPOSITIVO_DISCO:$LIMITE_IO_READ \
        #         --device-write-bps $DISPOSITIVO_DISCO:$LIMITE_IO_WRITE \
        #         --device-read-iops $DISPOSITIVO_DISCO:$LIMITE_IOPS_READ \
        #         --device-write-iops $DISPOSITIVO_DISCO:$LIMITE_IOPS_WRITE \
        #         --cgroupns=host alpine-stress stress --hdd 1
        #     ;;
    esac
done

echo "Contenedores creados correctamente."