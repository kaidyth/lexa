mod commands;
mod data;
extern crate tokio;

mod built_info {
    include!(concat!(env!("OUT_DIR"), "/built.rs"));
}

use commands::state::SubCommand::*;

use async_once_cell::OnceCell;
use moka::future::Cache;
pub(crate) static CACHE: OnceCell<
    Option<Box<Cache<String, String, std::collections::hash_map::RandomState>>>,
> = OnceCell::new();
/// Vaulted storage secrets for your teams, apps, and servers.
#[tokio::main]
async fn main() {
    // Configure the cache
    crate::CACHE
        .get_or_init(async {
            return Some(Box::new(
                moka::future::Cache::builder()
                    .time_to_live(std::time::Duration::from_secs(7))
                    .max_capacity(10_000)
                    .build(),
            ));
        })
        .await;

    // Parse arguments with clap => config::Config struct
    let cfg = commands::state::get_config();

    match &cfg.cmd {
        Server(command) => command.run(&cfg).await,
        //Agent(command) => command.run(&cfg).await,
        //Cluster(command) => command.run(&cfg).await,
    }
}
