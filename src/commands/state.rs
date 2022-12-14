use crate::commands::*;
use clap::Parser;
use std::sync::Arc;

#[derive(clap::Subcommand, Debug, Clone)]
pub enum SubCommand {
    /// Starts the lexa server
    Server(server::server::Config),
    // Starts Lexa in agent mode
    //Agent(agent::AgentState),
    // Provides an endpoint for clustering LXD servers
    //Cluster(cluster::ClusterState),
}

#[derive(Debug, Parser, Clone)]
#[clap(author, version, about, long_about = None)]
pub struct State {
    /// Command to execute
    #[clap(subcommand)]
    pub cmd: SubCommand,
}

// Parsing command for clap to correctly build the configuration.
pub fn get_config() -> Arc<State> {
    return Arc::new(State::parse());
}
