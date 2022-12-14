use std::iter;
use std::str::FromStr;

use anyhow::anyhow;
use rustls::{Certificate, PrivateKey};
use std::io::BufReader;
use std::time::Duration;
use tokio::net::{TcpListener, UdpSocket};
use trust_dns_server::resolver::Name;
use trust_dns_server::ServerFuture;
use trust_dns_server::{
    authority::MessageResponseBuilder,
    client::rr::LowerName,
    proto::op::{Header, MessageType, OpCode, ResponseCode},
    server::{Request, RequestHandler, ResponseHandler, ResponseInfo},
};

use super::config::{ApplicationConfigDNS, ApplicationConfigHostPort, ApplicationConfigLXD};

#[derive(thiserror::Error, Debug)]
pub enum Error {
    #[error("Invalid OpCode {0:}")]
    InvalidOpCode(OpCode),
    #[error("Invalid MessageType {0:}")]
    InvalidMessageType(MessageType),
    #[error("Invalid Zone '{0:}'")]
    InvalidZone(LowerName),
    #[error("IO error: {0:}")]
    Io(#[from] std::io::Error),
    #[error("Data Error {0:}")]
    DataError(#[from] anyhow::Error),
}

#[derive(Clone, Debug)]
pub struct Handler {
    pub lxd: ApplicationConfigLXD,
}

impl Handler {
    /// Handle request, returning ResponseInfo if response was successfully sent, or an error.
    async fn handle_request<R: ResponseHandler>(
        &self,
        request: &Request,
        mut responder: R,
    ) -> Result<ResponseInfo, Error> {
        // Make sure the message and message types are query
        if request.op_code() != OpCode::Query {
            return Err(Error::InvalidOpCode(request.op_code()));
        }

        if request.message_type() != MessageType::Query {
            return Err(Error::InvalidMessageType(request.message_type()));
        }

        let lexa_zone = match Name::from_str(&self.lxd.suffix) {
            Ok(zone) => LowerName::from(zone),
            Err(_) => return Err(Error::DataError(anyhow!("Requested DNS Zone is invalid."))),
        };

        let name = match request.query().name() {
            name if lexa_zone.zone_of(name) => name,
            name => return Err(Error::InvalidZone(name.clone())),
        };

        let builder = MessageResponseBuilder::from_message_request(request);
        let mut header = Header::response_from_request(request.header());
        header.set_authoritative(true);

        // Get the string name and return it without the suffix.
        let q_name = {
            let n = name.to_string().chars().rev().collect::<String>();
            let nn = match n
                .as_str()
                .char_indices()
                .skip((self.lxd.suffix.len() + 2))
                .next()
            {
                Some((pos, _)) => &n[pos..],
                None => "",
            };

            nn.to_string().chars().rev().collect::<String>()
        };

        match crate::data::Query::new(q_name, self.lxd.clone())
            .get_rdata_for_query(request.query().name().into(), request.query().query_type())
            .await
        {
            Ok(ips) => {
                let response = builder.build(header, ips.iter(), &[], &[], &[]);
                return Ok(responder.send_response(response).await?);
            }
            Err(_) => {
                header.set_response_code(ResponseCode::ServFail);
                let r = builder.build(header, &[], &[], &[], &[]);
                return Ok(responder.send_response(r).await.unwrap());
            }
        };
    }
}

#[async_trait::async_trait]
impl RequestHandler for Handler {
    async fn handle_request<R: ResponseHandler>(
        &self,
        request: &Request,
        mut response: R,
    ) -> ResponseInfo {
        match self.handle_request(request, response.clone()).await {
            Ok(info) => info,
            Err(error) => {
                tracing::error!("Error in RequestHandler: {error}");

                // Return SRV Fail
                let builder = MessageResponseBuilder::from_message_request(request);
                let mut header = Header::response_from_request(request.header());
                header.set_authoritative(true);
                header.set_response_code(ResponseCode::ServFail);
                let r = builder.build(header, &[], &[], &[], &[]);
                return response.send_response(r).await.unwrap();
            }
        }
    }
}

pub struct Server {
    pub server: ServerFuture<Handler>,
}

impl Server {
    pub async fn init(
        config: ApplicationConfigDNS,
        lxd: ApplicationConfigLXD,
    ) -> Result<Self, anyhow::Error> {
        let handler = Handler { lxd };
        let mut server = ServerFuture::new(handler);

        // Register the UDP listener
        let udp = Self::get_bind(config.bind);
        match UdpSocket::bind(&udp).await {
            Ok(socket) => {
                server.register_socket(socket);
            }
            Err(_) => return Err(anyhow!("Unable to setup UDP Listener")),
        };

        // Optionally setup the QUIC listener
        match config.quic {
            Some(config) => {
                let conn = Self::get_bind(config.bind);
                match UdpSocket::bind(&conn).await {
                    Ok(socket) => {
                        let certificate = match Self::get_certificate(config.certificate) {
                            Ok(cert) => cert,
                            Err(error) => return Err(error),
                        };

                        let key = match Self::get_privkey(config.key) {
                            Ok(key) => key,
                            Err(error) => return Err(error),
                        };

                        let _ = server.register_quic_listener(
                            socket,
                            Duration::from_secs(3),
                            (certificate, key),
                            config.hostname,
                        );
                        tracing::info!("Setup QUIC server.")
                    }
                    Err(_) => return Err(anyhow!("Unable to setup QUIC listener")),
                };
            }
            None => tracing::info!("Not setting up QUIC server."),
        };

        // Optionally setup the DoH listener
        match config.dot {
            Some(config) => {
                let conn = Self::get_bind(config.bind);
                match TcpListener::bind(&conn).await {
                    Ok(socket) => {
                        let certificate = match Self::get_certificate(config.certificate) {
                            Ok(cert) => cert,
                            Err(error) => return Err(error),
                        };

                        let key = match Self::get_privkey(config.key) {
                            Ok(key) => key,
                            Err(error) => return Err(error),
                        };

                        let _ = server.register_tls_listener(
                            socket,
                            Duration::from_secs(3),
                            (certificate, key),
                        );
                        tracing::info!("Setup DoT server.")
                    }
                    Err(_) => return Err(anyhow!("Unable to setup DoT listener")),
                };
            }
            None => tracing::info!("Not setting up DoT server."),
        };

        // Optionally setup the DoH listener
        match config.doh {
            Some(config) => {
                let conn = Self::get_bind(config.bind);
                match TcpListener::bind(&conn).await {
                    Ok(socket) => {
                        let certificate = match Self::get_certificate(config.certificate) {
                            Ok(cert) => cert,
                            Err(error) => return Err(error),
                        };

                        let key = match Self::get_privkey(config.key) {
                            Ok(key) => key,
                            Err(error) => return Err(error),
                        };

                        let hostname = match config.hostname {
                            Some(hostname) => hostname,
                            None => return Err(anyhow!("Hostname not set in configuration")),
                        };

                        let _ = server.register_https_listener(
                            socket,
                            Duration::from_secs(3),
                            (certificate, key),
                            hostname,
                        );
                        tracing::info!("Setup DoT server.")
                    }
                    Err(_) => return Err(anyhow!("Unable to setup DoT listener")),
                };
            }
            None => tracing::info!("Not setting up DoT server."),
        };

        Ok(Self { server })
    }

    /// Returns the bind configuration for a given host block
    fn get_bind(config: ApplicationConfigHostPort) -> String {
        return String::from(format!("{}:{}", config.host, config.port));
    }

    /// Returns the RusTLS Certificate for a given file
    fn get_certificate(file: String) -> Result<Vec<Certificate>, anyhow::Error> {
        match std::fs::File::open(file) {
            Ok(f) => {
                let mut reader = BufReader::new(f);

                let mut certs: Vec<Certificate> = Vec::<Certificate>::new();
                for item in iter::from_fn(|| rustls_pemfile::read_one(&mut reader).transpose()) {
                    match item.unwrap() {
                        rustls_pemfile::Item::X509Certificate(cert) => {
                            certs.push(rustls::Certificate(cert))
                        }
                        _ => tracing::warn!("Unhandled item in certificate chain."),
                    }
                }

                return Ok(certs);
            }
            Err(error) => return Err(anyhow!(error.to_string())),
        }
    }

    /// Returns the RusTLS PrivateKey for a given file
    fn get_privkey(file: String) -> Result<PrivateKey, anyhow::Error> {
        match std::fs::File::open(file) {
            Ok(f) => {
                let mut reader = BufReader::new(f);

                for item in iter::from_fn(|| rustls_pemfile::read_one(&mut reader).transpose()) {
                    match item.unwrap() {
                        rustls_pemfile::Item::RSAKey(key) => return Ok(rustls::PrivateKey(key)),
                        rustls_pemfile::Item::PKCS8Key(key) => return Ok(rustls::PrivateKey(key)),
                        rustls_pemfile::Item::ECKey(key) => return Ok(rustls::PrivateKey(key)),
                        _ => tracing::warn!("Unhandled item in certificate chain."),
                    }
                }

                return Err(anyhow!("Private key not found in file."));
            }
            Err(error) => return Err(anyhow!(error.to_string())),
        }
    }
}
