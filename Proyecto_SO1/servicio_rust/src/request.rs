use reqwest::Client;
use crate::process::{LogProcess};
use std::error::Error;
use serde::Deserialize;

const API_URL: &str = "http://localhost:8000";

#[derive(Deserialize)]
struct ErrorMessages {
    message: String,
}

pub async fn send_process(process_list: Vec<LogProcess>, url: &str) -> Result<(), Box<dyn Error>> {
    let json_body = serde_json::to_string(&process_list)?;
    let client = Client::new();
    let full_url = format!("{}/{}", API_URL, url);
    let res = client.post(full_url)
        .body(json_body)
        .send()
        .await?;

    if res.status().is_success() {
        println!("Logs enviados correctamente");
    } else {
        println!("Error al enviar procesos {}", res.status());
    }
    Ok(())
}

//Funcion para generar grafica PENDIENTE
/*
pub async fn gen_graph(url: &str) -> Result<(), Box<dyn Error>> {
    let client = Client::new();
    let full_url = format!("{}/{}", API_URL, url);
    let res = client.get(full_url)
        .send()
        .await?;

    if res.status().is_success() {
        println!("Grafica generada correctamente");
    } else {
        let response = res.text().await?;
        let error_message: ErrorMessages = serde_json::from_str(&response)?;
        println!("Error al generar la grafica: {}", error_message.message);
    }
    Ok(())
}
*/ 