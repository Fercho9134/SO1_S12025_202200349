use actix_web::{post, web, HttpResponse};
use crate::models::{WeatherTweet, ApiError};
use crate::services::http_client::HttpClient;

#[post("/input")]
pub async fn handle_input(
    tweet: web::Json<WeatherTweet>,
    http_client: web::Data<HttpClient>,
) -> Result<HttpResponse, ApiError> {
    log::info!("Received tweet: {:?}", tweet);
    
    http_client.forward_to_go(tweet.into_inner()).await?;
    
    Ok(HttpResponse::Ok().json("Tweet processed successfully"))
}