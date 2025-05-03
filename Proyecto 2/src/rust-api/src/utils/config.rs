use config::Config;
use serde::Deserialize;

#[derive(Debug, Deserialize)]
pub struct AppConfig {
    pub server_addr: String,
    pub go_service_url: String,
    pub max_concurrent_requests: usize,
}

impl AppConfig {
    pub fn load() -> Result<Self, config::ConfigError> {
        let settings = Config::builder()
            .add_source(config::File::with_name("./config/default.toml"))
            .add_source(config::Environment::with_prefix("APP"))
            .build()?;

        settings.try_deserialize()
    }
}