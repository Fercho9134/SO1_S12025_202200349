#!/bin/bash

# Definir las tareas cron
cronjob="* * * * * /bin/bash -c '/home/fernando/Escritorio/Proyecto_SO1/script/crear_contenedores.sh >> /home/fernando/cron.log 2>&1; sleep 30;'"

# Limpiar crontab y agregar solo la nueva tarea
echo "$cronjob" | crontab -
