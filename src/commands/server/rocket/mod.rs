use rocket::{self, config::LogLevel, figment::Figment, routes};

use super::config::{ApplicationConfigLXD, ApplicationConfigTLS};
use anyhow::anyhow;

mod routes;

pub struct Server {
    config: ApplicationConfigTLS,
    lxd: ApplicationConfigLXD,
    log_level: String,
}

impl Server {
    /// Initialize the server with configuration data
    pub fn init(
        config: ApplicationConfigTLS,
        lxd: ApplicationConfigLXD,
        log_level: String,
    ) -> Self {
        return Self {
            config,
            lxd,
            log_level,
        };
    }

    /// Starts the Rocket server
    pub async fn run(&self) -> Result<bool, anyhow::Error> {
        match self.get_rocket_config() {
            Ok(config) => {
                let rocket = rocket::custom(config)
                    .manage(self.lxd.clone())
                    .mount("/containers", routes![routes::list]);

                if let Ok(ignite) = rocket.ignite().await {
                    match ignite.launch().await {
                        Ok(_) => return Ok(true),
                        Err(error) => return Err(anyhow!(error)),
                    }
                }

                return Err(anyhow!("Could not start TLS server."));
            }
            Err(error) => return Err(anyhow!("Unable to start TLS web server.")),
        }
    }

    /// Returns the appropriate log level for Rocket.rs
    fn get_rocket_log_level(&self) -> LogLevel {
        match self.log_level.as_str() {
            "info" => LogLevel::Normal,
            "trace" => LogLevel::Debug,
            "debug" => LogLevel::Normal,
            "error" => LogLevel::Critical,
            "warn" => LogLevel::Critical,
            _ => LogLevel::Off,
        }
    }

    /// Generates the rocket configuration
    fn get_rocket_config(&self) -> Result<Figment, anyhow::Error> {
        if !std::path::Path::new(&self.config.certificate).exists()
            || !std::path::Path::new(&self.config.key).exists()
        {
            return Err(anyhow!("TLS Certificate or Key is not valid"));
        }

        let figment = rocket::Config::figment()
            .merge(("profile", rocket::figment::Profile::new("release")))
            .merge(("ident", false))
            .merge(("log_level", self.get_rocket_log_level()))
            .merge(("port", &self.config.bind.port))
            .merge(("address", &self.config.bind.host))
            .merge(("tls.certs", &self.config.certificate))
            .merge(("tls.key", &self.config.key));

        return Ok(figment);
    }
}
