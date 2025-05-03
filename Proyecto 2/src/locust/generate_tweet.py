import json
import random

def generate_weather_data(num_records):

    countries = ["GT", "BR", "ESP", "EEUU", "MX", "AR", "CO", "PE", "CL", "CA"]
    weather_types = ["lluvioso", "nubloso", "soleado"]
    description_bases = [
        "Reporte del clima actual en ",
        "Condiciones meteorológicas para ",
        "Estado del tiempo en ",
        "Pronóstico a corto plazo para "
    ]

    weather_reports = []

    for i in range(num_records):
        country = random.choice(countries)
        weather = random.choice(weather_types)
        description_base = random.choice(description_bases)

        # Variar ligeramente la descripción
        if description_base.endswith("en "):
            description = f"{description_base}{country}."
        else:
             description = f"{description_base}{country}."

        report = {
            "description": description,
            "country": country,
            "weather": weather
        }
        weather_reports.append(report)

    return weather_reports


num_records = 10000

# Generar los datos
weather_data = generate_weather_data(num_records)

# Guarda el resultado en un archivo JSON
# Se creará un directorio 'test' si no existe
import os
output_dir = "./"
if not os.path.exists(output_dir):
    os.makedirs(output_dir)

output_filename = os.path.join(output_dir, "weather_reports.json")

with open(output_filename, "w", encoding='utf-8') as json_file:
    json.dump(weather_data, json_file, indent=4, ensure_ascii=False)

print(f"JSON generado y guardado en '{output_filename}'")