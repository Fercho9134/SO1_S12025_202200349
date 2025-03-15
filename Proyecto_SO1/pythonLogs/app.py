from fastapi import FastAPI, HTTPException
from fastapi.responses import JSONResponse
import json
import os
from typing import List, Dict

app = FastAPI()

# Ruta para recibir logs y almacenarlos
@app.post("/logs")
def post_logs(logs_proc: List[Dict]):  # <--- Recibimos una lista de diccionarios
    logs_file = '/logs/logs.json'  # Guardar en volumen mapeado en Docker
    
    # Verificamos si el archivo logs.json existe
    if os.path.exists(logs_file):
        # Leemos el archivo logs.json
        with open(logs_file, 'r') as file:
            existing_logs = json.load(file)
    else:
        # Si no existe, iniciamos una lista vacía
        existing_logs = []

    # Agregamos los nuevos logs a los existentes
    existing_logs.extend(logs_proc)

    # Guardamos la lista de logs en el archivo logs.json
    with open(logs_file, 'w') as file:
        json.dump(existing_logs, file, indent=4)

    return {"received": len(logs_proc), "total_logs": len(existing_logs)}

# Ruta para obtener los logs
@app.get("/logs")
def get_logs():
    logs_file = '/logs/logs.json'  # Ruta donde se almacenan los logs en el volumen
    
    if os.path.exists(logs_file):
        # Leemos los logs desde el archivo
        with open(logs_file, 'r') as file:
            existing_logs = json.load(file)
    else:
        existing_logs = []
        return JSONResponse(content={"message": "No logs found"}, status_code=404)
    
    if not existing_logs:
        return JSONResponse(content={"message": "No logs found"}, status_code=404)
    
    return {"logs": existing_logs}