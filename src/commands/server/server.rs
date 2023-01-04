use crate::commands::server::dns;

use super::{config::ApplicationConfig, rocket};
use clap::Parser;
use serde_json::Value;

use super::super::state::State as StateConfig;
use anyhow::anyhow;
use faccess::PathExt;
use std::{path::Path, process::exit};
use tracing_appender::non_blocking::{NonBlocking, WorkerGuard};
use tracing_subscriber::fmt::SubscriberBuilder;

/// Starts the Homemaker API Server
#[derive(Debug, Parser, Clone)]
#[clap(author, version, about, long_about = None)]
pub struct Config {
    // Path to homemaker configuration file
    #[clap(
        short,
        long,
        value_parser,
        required = false,
        default_value = "lexa.hcl"
    )]
    pub config: String,

    #[clap(skip)]
    pub config_data: ApplicationConfig,
}

impl Config {
    pub async fn run<'a>(&'a self, cfg: &StateConfig) {
        match self.get_config_file() {
            Ok(hcl) => {
                let config = Config {
                    config: self.config.clone(),
                    config_data: hcl,
                };
                config.serve_internal(cfg).await;
            }
            Err(error) => {
                println!("{}", error);
                exit(1);
            }
        };
    }

    async fn serve_internal<'a>(&'a self, _cfg: &StateConfig) {
        // Setup the logger
        let level = self.config_data.get_tracing_log_level();
        let out = &self.config_data.log.out;

        let subscriber: SubscriberBuilder = tracing_subscriber::fmt();
        let non_blocking: NonBlocking;
        let _guard: WorkerGuard;

        match out.to_lowercase().as_str() {
            "stdout" => {
                (non_blocking, _guard) = tracing_appender::non_blocking(std::io::stdout());
            }
            _ => {
                let path = Path::new(out);
                if !path.exists() || !path.writable() {
                    println!("{} doesn't exist or is not writable", out);
                    exit(1);
                }
                let file_appender = tracing_appender::rolling::daily(out, "lexa .log");
                (non_blocking, _guard) = tracing_appender::non_blocking(file_appender);
            }
        };

        subscriber
            .with_writer(non_blocking)
            .with_max_level(level)
            .with_level(true)
            .with_line_number(level == tracing::Level::TRACE)
            .with_file(level == tracing::Level::TRACE)
            .compact()
            .init();

        // Setup tokio threads for the HTTP server and the DNS server
        let mut tasks = Vec::new();

        let rocket_server = rocket::Server::init(
            self.config_data.tls.clone(),
            self.config_data.lxd.clone(),
            self.config_data.log.level.clone(),
        );

        let rocket = tokio::spawn(async move {
            #[allow(unused_must_use)]
            {
                rocket_server.run().await;
            };
        });

        tasks.push(rocket);

        let server =
            match dns::Server::init(self.config_data.dns.clone(), self.config_data.lxd.clone())
                .await
            {
                Ok(server) => server,
                Err(error) => {
                    println!("{}", error.to_string());
                    exit(1);
                }
            };

        let dns = tokio::spawn(async move {
            #[allow(unused_must_use)]
            {
                server.server.block_until_done().await;
            }
        });
        tasks.push(dns);

        for task in tasks {
            #[allow(unused_must_use)]
            {
                task.await;
            }
        }
    }

    /// Retrieves the configuration file from disk
    fn get_config_file<'a>(&'a self) -> std::result::Result<ApplicationConfig, anyhow::Error> {
        if let Ok(config) = std::fs::read_to_string(&self.config) {
            if let Ok(hcl) = hcl::from_str::<Value>(&config.as_str()) {
                let app_config: Result<ApplicationConfig, serde_json::Error> =
                    serde_json::from_value(hcl.get("server").unwrap().to_owned());
                if app_config.is_ok() {
                    let acr = app_config.unwrap();
                    return Ok::<ApplicationConfig, anyhow::Error>(acr);
                } else {
                    return Err(anyhow!(app_config.unwrap_err()));
                }
            }
        }

        return Err(anyhow!("Unable to parse config file."));
    }
}
