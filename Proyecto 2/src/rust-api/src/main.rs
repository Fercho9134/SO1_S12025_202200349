mod handlers;
mod models;
mod services;
mod utils;

use actix_web::{App, HttpServer, web};
use handlers::handle_input;
use crate::services::http_client::HttpClient;
use std::sync::Arc;
use utils::config::AppConfig;

#[actix_web::main]
async fn main() -> std::io::Result<()> {
    env_logger::init();

    let config = AppConfig::load().expect("Failed to load configuration");
    
    log::info!("Starting server at {}", config.server_addr);
    log::info!("Forwarding requests to Go service at {}", config.go_service_url);

    let http_client = Arc::new(HttpClient::new(config.go_service_url));

    HttpServer::new(move || {
        App::new()
            .app_data(web::Data::from(http_client.clone()))
            .service(handle_input)
    })
    .bind(config.server_addr)?
    .workers(config.max_concurrent_requests)
    .run()
    .await
}