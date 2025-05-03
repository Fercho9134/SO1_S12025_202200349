import json
from locust import HttpUser, task, between

class WeatherTraffic(HttpUser):
    host = "https://34.27.224.230.nip.io"
    verify = False

    headers = {
        "Content-Type": "application/json",
        "Accept": "*/*",
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/58.0.3029.110 Safari/537.3",
        "Accept-Encoding": "gzip, deflate, br",
        "Connection": "keep-alive"
    }

    wait_time = between(1, 5)

    def on_start(self):
        self.client.verify = False
        with open('weather_reports.json', 'r', encoding='utf-8') as file:
            self.weather_data = json.load(file)

    @task
    def send_traffic(self):
        for report in self.weather_data:
            response = self.client.post("/input", json=report, headers = self.headers, verify=False)
            print(f"Response status code: {response.text}")
            