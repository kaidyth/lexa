use std::{io::Read, net::IpAddr, str::FromStr};

use anyhow::anyhow;
use serde::{Deserialize, Serialize};
use tracing::Level;

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct ApplicationConfig {
    #[serde(default)]
    pub lxd: ApplicationConfigLXD,
    #[serde(default)]
    pub tls: ApplicationConfigTLS,
    #[serde(default)]
    pub dns: ApplicationConfigDNS,
    #[serde(default)]
    pub log: ApplicationConfigLogger,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct ApplicationConfigLXD {
    #[serde(default)]
    pub suffix: String,
    pub bind: ApplicationConfigHostPort,
    #[serde(default)]
    pub certificate: String,
    #[serde(default)]
    pub key: String,
}

impl From<&rocket::State<ApplicationConfigLXD>> for ApplicationConfigLXD {
    fn from(item: &rocket::State<ApplicationConfigLXD>) -> Self {
        ApplicationConfigLXD {
            suffix: item.suffix.clone(),
            certificate: item.certificate.clone(),
            key: item.key.clone(),
            bind: ApplicationConfigHostPort {
                host: item.bind.host.clone(),
                port: item.bind.port.clone(),
            },
        }
    }
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct ApplicationConfigTLS {
    pub bind: ApplicationConfigHostPort,
    #[serde(default)]
    pub so_reuse_port: bool,
    #[serde(default)]
    pub certificate: String,
    #[serde(default)]
    pub key: String,
    #[serde(default)]
    pub mtls: Option<ApplicationConfigTLSMTLS>,
    #[serde(default)]
    pub hostname: Option<String>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct ApplicationConfigTLSMTLS {
    #[serde(default)]
    pub ca_certificate: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct ApplicationConfigDNS {
    pub bind: ApplicationConfigHostPort,
    #[serde(default)]
    pub quic: Option<ApplicationConfigQuic>,
    #[serde(default)]
    pub dot: Option<ApplicationConfigTLS>,
    #[serde(default)]
    pub doh: Option<ApplicationConfigTLS>,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct ApplicationConfigQuic {
    pub bind: ApplicationConfigHostPort,
    #[serde(default)]
    pub hostname: String,
    #[serde(default)]
    pub certificate: String,
    #[serde(default)]
    pub key: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct ApplicationConfigHostPort {
    #[serde(default)]
    pub port: u32,
    #[serde(default)]
    pub host: String,
}

#[derive(Serialize, Deserialize, Debug, Clone)]
pub struct ApplicationConfigLogger {
    #[serde(default)]
    pub level: String,
    #[serde(default)]
    pub out: String,
}

/// Default values for ApplicationConfig struct
impl Default for ApplicationConfig {
    fn default() -> Self {
        ApplicationConfig {
            lxd: ApplicationConfigLXD {
                suffix: String::from("lexa"),
                bind: ApplicationConfigHostPort {
                    port: 8443,
                    host: String::from("127.0.0.1"),
                },
                certificate: String::from("lexa.crt"),
                key: String::from("lexa.key"),
            },
            tls: ApplicationConfigTLS {
                bind: ApplicationConfigHostPort {
                    port: 18053,
                    host: String::from("127.0.0.1"),
                },
                so_reuse_port: false,
                certificate: String::from("server.crt"),
                key: String::from("server.key"),
                mtls: None,
                hostname: None,
            },
            dns: ApplicationConfigDNS {
                bind: ApplicationConfigHostPort {
                    port: 18053,
                    host: String::from("127.0.0.1"),
                },
                doh: None,
                quic: None,
                dot: None,
            },
            log: ApplicationConfigLogger {
                level: String::from("info"),
                out: String::from("stdout"),
            },
        }
    }
}

impl Default for ApplicationConfigLXD {
    fn default() -> Self {
        ApplicationConfigLXD {
            suffix: String::from("lexa"),
            bind: ApplicationConfigHostPort {
                port: 8443,
                host: String::from("127.0.0.1"),
            },
            certificate: String::from("lexa.crt"),
            key: String::from("lexa.key"),
        }
    }
}

impl ApplicationConfigLXD {
    /// Returns a reqwest certificate
    fn get_certificate(&self) -> Result<reqwest::Certificate, anyhow::Error> {
        let mut buf = Vec::new();
        match std::fs::File::open(&self.certificate) {
            Ok(mut file) => match file.read_to_end(&mut buf) {
                Ok(_) => match reqwest::Certificate::from_pem(&buf) {
                    Ok(cert) => Ok(cert),
                    Err(error) => {
                        tracing::error!("{}", error.to_string());
                        return Err(anyhow!("Unable to retrieve certificate."));
                    }
                },
                Err(error) => {
                    tracing::error!("{}", error.to_string());
                    return Err(anyhow!("Unable to retrieve certificate."));
                }
            },
            Err(error) => {
                tracing::error!("{}", error.to_string());
                return Err(anyhow!("Unable to retrieve mutual client certificate."));
            }
        }
    }

    /// Returns a reqwest identity
    fn get_identity(&self) -> Result<reqwest::Identity, anyhow::Error> {
        let mut buf = Vec::new();
        match std::fs::File::open(&self.certificate) {
            Ok(mut file) => match file.read_to_end(&mut buf) {
                Ok(_) => {}
                Err(error) => {
                    tracing::error!("{}", error.to_string());
                    return Err(anyhow!("Unable to retrieve private key."));
                }
            },
            Err(error) => {
                tracing::error!("{}", error.to_string());
                return Err(anyhow!("Unable to retrieve mutual client key."));
            }
        }

        match std::fs::File::open(&self.key) {
            Ok(mut file) => match file.read_to_end(&mut buf) {
                Ok(_) => {}
                Err(error) => {
                    tracing::error!("{}", error.to_string());
                    return Err(anyhow!("Unable to retrieve certificate."));
                }
            },
            Err(error) => {
                tracing::error!("{}", error.to_string());
                return Err(anyhow!("Unable to retrieve mutual client key."));
            }
        }

        match reqwest::Identity::from_pem(&buf) {
            Ok(identity) => Ok(identity),
            Err(error) => {
                tracing::error!("{}", error.to_string());
                return Err(anyhow!("Unable to construct identity."));
            }
        }
    }

    /// Returns a functional FQDN because RusTLS requires a FQDN instead of an IP connection
    pub fn get_fqdn(&self) -> String {
        match IpAddr::from_str(&self.bind.host) {
            Ok(_) => String::from("local.lexa.kaidyth.com"),
            Err(_) => String::from(&self.bind.host),
        }
    }

    pub fn get_client(&self) -> Result<reqwest::Client, anyhow::Error> {
        let cert = match self.get_certificate() {
            Ok(cert) => cert,
            Err(error) => return Err(error),
        };

        let key = match self.get_identity() {
            Ok(key) => key,
            Err(error) => return Err(error),
        };

        let mut builder = reqwest::Client::builder()
            .use_rustls_tls()
            .tls_built_in_root_certs(false)
            .identity(key)
            .add_root_certificate(cert)
            // Workaround for https://github.com/rustls/rustls/issues/184
            .danger_accept_invalid_certs(true)
            .timeout(std::time::Duration::new(5, 0));

        // If an IP Address is provided, remap it to our magic local hostname, otherwise return the FQDN directly.
        builder = match IpAddr::from_str(&self.bind.host) {
            Ok(_) => builder.resolve_to_addrs(
                "local.lexa.kaidyth.com",
                &[format!("{}:{}", self.bind.host, self.bind.port)
                    .parse()
                    .unwrap()],
            ),
            Err(_) => builder,
        };

        match builder.build() {
            Ok(client) => Ok(client),
            Err(error) => Err(anyhow!(error.to_string())),
        }
    }
}

impl Default for ApplicationConfigTLS {
    fn default() -> Self {
        ApplicationConfigTLS {
            bind: ApplicationConfigHostPort {
                port: 18443,
                host: String::from("127.0.0.1"),
            },
            so_reuse_port: false,
            certificate: String::from("server.crt"),
            key: String::from("server.key"),
            mtls: None,
            hostname: None,
        }
    }
}

impl Default for ApplicationConfigDNS {
    fn default() -> Self {
        ApplicationConfigDNS {
            bind: ApplicationConfigHostPort {
                port: 18053,
                host: String::from("127.0.0.1"),
            },
            doh: None,
            quic: None,
            dot: None,
        }
    }
}

impl Default for ApplicationConfigLogger {
    fn default() -> Self {
        ApplicationConfigLogger {
            level: String::from("info"),
            out: String::from("stdout"),
        }
    }
}

impl ApplicationConfig {
    pub fn get_tracing_log_level<'a>(&'a self) -> tracing::Level {
        match self.log.level.as_str() {
            "info" => Level::INFO,
            "trace" => Level::TRACE,
            "debug" => Level::DEBUG,
            "warn" => Level::WARN,
            _ => Level::ERROR,
        }
    }
}
