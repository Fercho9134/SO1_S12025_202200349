use crate::models::{ApiError, WeatherTweet};
use reqwest::Client;
use std::time::Duration;

pub struct HttpClient {
    client: Client,
    go_service_url: String,
}

impl HttpClient {
    pub fn new(go_service_url: String) -> Self {
        let client = Client::builder()
            .timeout(Duration::from_secs(5))
            .build()
            .expect("Failed to create HTTP client");

        Self {
            client,
            go_service_url,
        }
    }

    pub async fn forward_to_go(&self, tweet: WeatherTweet) -> Result<(), ApiError> {
        self.client
            .post(&self.go_service_url)
            .json(&tweet)
            .send()
            .await
            .map_err(|e| ApiError::RequestError(e.to_string()))?
            .error_for_status()
            .map_err(|e| ApiError::RequestError(e.to_string()))?;

        Ok(())
    }
}