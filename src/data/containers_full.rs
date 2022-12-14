use serde::Deserialize;
use serde::Serialize;
use std::collections::HashMap;

#[derive(Default, Debug, Clone, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "camelCase")]
pub struct Root {
    #[serde(rename = "type")]
    pub type_field: String,
    pub status: String,
    #[serde(rename = "status_code")]
    pub status_code: i64,
    pub operation: String,
    #[serde(rename = "error_code")]
    pub error_code: i64,
    pub error: String,
    pub metadata: Vec<Metadaum>,
}

#[derive(Default, Debug, Clone, PartialEq, Serialize, Deserialize)]

pub struct Metadaum {
    pub architecture: String,
    pub config: HashMap<String, String>,
    pub expanded_config: HashMap<String, String>,
    pub devices: HashMap<String, serde_json::Value>,
    pub expanded_devices: HashMap<String, serde_json::Value>,
    pub name: String,
    pub stateful: bool,
    pub ephemeral: bool,
    pub description: String,
    pub status: String,
    pub status_code: i32,
    pub created_at: String,
    pub last_used_at: String,
    pub location: String,
    #[serde(rename = "type")]
    pub type_field: String,
    pub project: String,
    pub state: State,
}

#[derive(Default, Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Devices {}

#[derive(Default, Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct ExpandedDevices {
    pub data: HashMap<String, serde_json::Value>,
}

#[derive(Default, Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct State {
    pub status: String,
    pub status_code: i64,
    pub disk: Disk,
    pub memory: Memory,
    pub network: HashMap<String, Network>,
    pub pid: i64,
    pub processes: i64,
    pub cpu: Cpu,
}

#[derive(Default, Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Disk {}

#[derive(Default, Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Memory {
    pub usage: i64,
    pub usage_peak: i64,
    pub swap_usage: i64,
    pub swap_usage_peak: i64,
}

#[derive(Default, Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Network {
    pub addresses: Vec<Address>,
    pub counters: Counters,
    pub hwaddr: String,
    pub host_name: String,
    pub mtu: i64,
    pub state: String,
    #[serde(rename = "type")]
    pub type_field: String,
}

#[derive(Default, Debug, Clone, PartialEq, Serialize, Deserialize)]
pub enum NetworkFamily {
    #[serde(rename = "inet6")]
    INet6,
    #[default]
    #[serde(rename = "inet")]
    Inet,
}
#[derive(Default, Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Address {
    pub family: NetworkFamily,
    pub address: String,
    pub netmask: String,
    pub scope: String,
}

#[derive(Default, Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Counters {
    pub bytes_received: i64,
    pub bytes_sent: i64,
    pub packets_received: i64,
    pub packets_sent: i64,
    pub errors_received: i64,
    pub errors_sent: i64,
    pub packets_dropped_outbound: i64,
    pub packets_dropped_inbound: i64,
}

#[derive(Default, Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Cpu {
    pub usage: i64,
}
