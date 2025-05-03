use serde::{Deserialize, Serialize};
use thiserror::Error;

#[derive(Debug, Serialize, Deserialize)]
pub struct WeatherTweet {
    pub description: String,
    pub country: String,
    pub weather: WeatherType,
}

#[derive(Debug, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum WeatherType {
    Lluvioso,
    Nubloso,
    Soleado,
}

#[derive(Error, Debug)]
pub enum ApiError {
    #[error("Invalid weather type")]
    InvalidWeatherType,
    #[error("Request error: {0}")]
    RequestError(String),
    #[error("Internal server error")]
    InternalError,
}

impl actix_web::ResponseError for ApiError {
    fn error_response(&self) -> actix_web::HttpResponse {
        match self {
            ApiError::InvalidWeatherType => actix_web::HttpResponse::BadRequest().json("Invalid weather type"),
            ApiError::RequestError(msg) => actix_web::HttpResponse::BadGateway().json(msg),
            ApiError::InternalError => actix_web::HttpResponse::InternalServerError().json("Internal server error"),
        }
    }
}