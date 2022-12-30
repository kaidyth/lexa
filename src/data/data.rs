use std::{collections::HashMap, str::FromStr};

use super::containers_full::Metadaum;
use crate::{commands::server::config::ApplicationConfigLXD, data::containers_full::NetworkFamily};
use anyhow::anyhow;
use dns_lookup::lookup_host;
use moka::future::Cache;
use serde::{Deserialize, Serialize};
use trust_dns_server::{
    proto::rr::{rdata::SRV, RData, Record, RecordType},
    resolver::Name,
};

#[derive(Debug, Clone)]
pub struct Instance {
    pub name: String,
    pub data: crate::data::containers_full::Metadaum,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InstanceService {
    pub interface: Option<String>,
    pub name: String,
    pub port: i32,
    pub proto: String,
    pub tags: Option<Vec<String>>,
}

#[derive(Debug, Clone)]
pub struct Query {
    pub name: String,
    config: ApplicationConfigLXD,
    query_pieces: Option<Vec<String>>,
}

#[derive(Debug, Clone, Eq, PartialEq)]
pub enum QueryType {
    Cluster,
    Interface,
    Service,
    Container,
}

#[derive(Debug, Clone, Eq, PartialEq)]
pub enum ServiceQueryType {
    RFC2782,
    Tagged,
}

#[derive(Debug, Clone, Eq, PartialEq)]
pub struct ServiceQueryData {
    pub tag: Option<String>,
    pub service: Option<String>,
    pub protocol: Option<String>,
}

#[derive(Debug, Clone, Eq, PartialEq)]
pub enum ServiceQueryProtocol {
    Tcp,
    Udp,
}

impl ServiceQueryData {
    /// Returns true if the data is a tag
    pub fn is_tag(&self) -> bool {
        return self.tag.is_some();
    }

    /// Returns retur if it is a service query
    pub fn is_service(&self) -> bool {
        return self.service.is_some();
    }

    /// Returns the protocol as an enum
    pub fn get_protocol(&self) -> Option<ServiceQueryProtocol> {
        match &self.protocol {
            Some(p) => match p.as_str() {
                "_tcp" => Some(ServiceQueryProtocol::Tcp),
                "_udp" => Some(ServiceQueryProtocol::Udp),
                _ => None,
            },
            None => None,
        }
    }
}
impl Query {
    /// Returns a new query
    pub fn new(name: String, config: ApplicationConfigLXD) -> Self {
        Self {
            name,
            config,
            query_pieces: None,
        }
    }

    /// Retrieves the cache
    async fn get_cache(&self) -> Result<&Box<Cache<String, String>>, anyhow::Error> {
        match crate::CACHE.get() {
            Some(cache) => match cache {
                Some(cache) => Ok(cache),
                None => return Err(anyhow!("Unable to retrieve cache.")),
            },
            None => return Err(anyhow!("Unable to retrieve cache.")),
        }
    }

    /// Returns all IPS address for the given query
    pub async fn get_rdata_for_query(
        &mut self,
        name: Name,
        q_type: RecordType,
    ) -> Result<Vec<Record>, anyhow::Error> {
        let mut records = Vec::<Record>::new();
        let instances = self.get_instances().await?;

        for i in instances {
            match self.get_query_type() {
                // Returns an interface specific response
                QueryType::Interface => {
                    let interface_name = self.get_interface_name();
                    let mut results = i.get_records(
                        &self.config.suffix,
                        q_type.clone(),
                        interface_name,
                        false,
                        None,
                    );
                    records.append(&mut results);
                }
                // Returns `eth0` or the first network interface provided by LXD API
                QueryType::Container => {
                    let interface_name = {
                        let interfaces = i.get_interfaces();
                        if interfaces.contains(&String::from("eth0")) {
                            Some(String::from("eth0"))
                        } else {
                            Some(String::from(interfaces[0].clone()))
                        }
                    };

                    records.append(&mut i.get_records(
                        &self.config.suffix,
                        q_type.clone(),
                        interface_name,
                        true,
                        None,
                    ))
                }
                // Return the cluster location
                QueryType::Cluster => {
                    let pieces = self.get_query_pieces();
                    let mut node: Option<String> = None;
                    if pieces.len() >= 3 {
                        let fqdn = pieces[0..pieces.len() - 2].join(".");
                        node = Some(fqdn);
                    }
                    match lookup_host(&i.data.location) {
                        Ok(ips) => match &q_type {
                            RecordType::CNAME => {
                                let n = Name::from_str(i.data.location.as_str()).unwrap();
                                let rdata = RData::CNAME(n.clone());
                                let mut rec = vec![Record::from_rdata(name.clone(), 3, rdata)];
                                records.append(&mut rec);
                            }
                            RecordType::A => {
                                for ip in ips {
                                    if ip.is_ipv4() {
                                        match &node {
                                            Some(node) => {
                                                if node == i.data.location.as_str() {
                                                    let interface_name = {
                                                        let interfaces = i.get_interfaces();
                                                        if interfaces
                                                            .contains(&String::from("eth0"))
                                                        {
                                                            Some(String::from("eth0"))
                                                        } else {
                                                            Some(String::from(
                                                                interfaces[0].clone(),
                                                            ))
                                                        }
                                                    };

                                                    records.append(&mut i.get_records(
                                                        &self.config.suffix,
                                                        q_type.clone(),
                                                        interface_name,
                                                        true,
                                                        Some(name.clone()),
                                                    ))
                                                }
                                            }
                                            None => {
                                                let rdata =
                                                    RData::A(ip.to_string().parse().unwrap());
                                                let mut rec = vec![Record::from_rdata(
                                                    name.clone(),
                                                    3,
                                                    rdata,
                                                )];
                                                records.append(&mut rec)
                                            }
                                        }
                                    }
                                }
                            }
                            RecordType::AAAA => {
                                for ip in ips {
                                    if ip.is_ipv6() {
                                        match &node {
                                            Some(node) => {
                                                if node == i.data.location.as_str() {
                                                    let interface_name = {
                                                        let interfaces = i.get_interfaces();
                                                        if interfaces
                                                            .contains(&String::from("eth0"))
                                                        {
                                                            Some(String::from("eth0"))
                                                        } else {
                                                            Some(String::from(
                                                                interfaces[0].clone(),
                                                            ))
                                                        }
                                                    };

                                                    records.append(&mut i.get_records(
                                                        &self.config.suffix,
                                                        q_type.clone(),
                                                        interface_name,
                                                        true,
                                                        Some(name.clone()),
                                                    ))
                                                }
                                            }
                                            None => {
                                                let rdata =
                                                    RData::AAAA(ip.to_string().parse().unwrap());
                                                let mut rec = vec![Record::from_rdata(
                                                    name.clone(),
                                                    3,
                                                    rdata,
                                                )];
                                                records.append(&mut rec)
                                            }
                                        }
                                    }
                                }
                            }
                            _ => {}
                        },
                        Err(_) => {}
                    };
                }
                // Handle Service Queries
                QueryType::Service => {
                    // Get the response type, and the associated data
                    let (service_query_type, service_query_data) =
                        self.get_service_type_and_data(q_type)?;

                    let query_proto = match service_query_data.get_protocol() {
                        Some(q) => q,
                        None => ServiceQueryProtocol::Tcp,
                    };

                    let service_name = match &service_query_data.service {
                        Some(s) => s.replace("_", ""),
                        None => String::from(""),
                    };

                    let tag = match &service_query_data.tag {
                        Some(t) => t.to_owned(),
                        None => String::from(""),
                    };

                    let interface_name = {
                        let interfaces = i.get_interfaces();
                        if interfaces.contains(&String::from("eth0")) {
                            String::from("eth0")
                        } else {
                            String::from(interfaces[0].clone())
                        }
                    };

                    match i.get_service_config() {
                        Some(service_config) => {
                            for service in service_config {
                                match service_query_type {
                                    ServiceQueryType::RFC2782 => {
                                        let proto = match service.proto.as_str() {
                                            "_tcp" => ServiceQueryProtocol::Tcp,
                                            "_udp" => ServiceQueryProtocol::Udp,
                                            _ => ServiceQueryProtocol::Tcp,
                                        };

                                        if service_query_data.is_tag() {
                                            // _<service>._<proto>.service.lexa
                                            let tags = match service.tags {
                                                Some(tags) => tags,
                                                None => Vec::<String>::new(),
                                            };

                                            if tags.contains(&tag) && query_proto.eq(&proto) {
                                                let n = Name::from_str(
                                                    format!(
                                                        "{}.if.{}.{}",
                                                        &interface_name,
                                                        &i.name,
                                                        &self.config.suffix
                                                    )
                                                    .as_str(),
                                                )
                                                .unwrap();
                                                let rdata = RData::SRV(SRV::new(
                                                    1,
                                                    1,
                                                    service.port as u16,
                                                    n,
                                                ));

                                                let mut rec = vec![Record::from_rdata(
                                                    name.clone(),
                                                    3,
                                                    rdata,
                                                )];
                                                records.append(&mut rec);
                                            }
                                        } else {
                                            // <tag>._<proto>.service.lexa
                                            if service_name == service.name
                                                && query_proto.eq(&proto)
                                            {
                                                let n = Name::from_str(
                                                    format!(
                                                        "{}.if.{}.{}",
                                                        &interface_name,
                                                        &i.name,
                                                        &self.config.suffix
                                                    )
                                                    .as_str(),
                                                )
                                                .unwrap();
                                                let rdata = RData::SRV(SRV::new(
                                                    1,
                                                    1,
                                                    service.port as u16,
                                                    n,
                                                ));

                                                let mut rec = vec![Record::from_rdata(
                                                    name.clone(),
                                                    3,
                                                    rdata,
                                                )];
                                                records.append(&mut rec);
                                            }
                                        }
                                    }
                                    ServiceQueryType::Tagged => {
                                        // This will be <tag>.<service>.service.lexa or <service>.lexa and we are going to return either an A or AAAA record
                                        // Unimplemented by design
                                    }
                                }
                            }
                        }
                        None => {} // If there is no service configuration then we shouldn't return anything
                    };
                }
            };
        }

        return Ok(records);
    }

    pub fn get_service_type_and_data(
        &mut self,
        q_type: RecordType,
    ) -> Result<(ServiceQueryType, ServiceQueryData), anyhow::Error> {
        if self.get_query_type() == QueryType::Service {
            let sqt = match q_type {
                RecordType::SRV => ServiceQueryType::RFC2782,
                _ => ServiceQueryType::Tagged,
            };

            let parts = self.get_query_pieces();
            let mut service_name: Option<String> = None;
            let mut whatever: Option<String> = None;

            // This is a containerless service query
            if parts[parts.len() - 1] == "service" {
                if parts.len() >= 2 {
                    service_name = Some(parts[parts.len() - 2].clone());
                }

                if parts.len() == 3 {
                    whatever = Some(parts[parts.len() - 3].clone());
                }

                match sqt {
                    ServiceQueryType::RFC2782 => {
                        let mut tag: Option<String> = None;
                        let mut proto: Option<String> = None;

                        if whatever.is_some() {
                            match whatever.as_ref().unwrap().chars().next() {
                                Some('_') => {
                                    proto = service_name.clone();
                                    tag = None;
                                    service_name = whatever.clone();
                                }
                                _ => {
                                    tag = whatever.clone();
                                    proto = service_name;
                                    service_name = None;
                                }
                            };
                        }
                        return Ok((
                            ServiceQueryType::RFC2782,
                            ServiceQueryData {
                                service: service_name,
                                tag: tag,
                                protocol: proto,
                            },
                        ));
                    }
                    ServiceQueryType::Tagged => {
                        return Ok((
                            ServiceQueryType::Tagged,
                            ServiceQueryData {
                                service: service_name,
                                tag: whatever,
                                protocol: None,
                            },
                        ))
                    }
                }
            } else {
                // This is a container specific query
                if parts.len() >= 3 {
                    service_name = Some(parts[parts.len() - 3].clone());
                }

                if parts.len() == 4 {
                    whatever = Some(parts[parts.len() - 4].clone());
                }

                match sqt {
                    ServiceQueryType::RFC2782 => {
                        let mut tag: Option<String> = None;
                        let mut proto: Option<String> = None;

                        if whatever.is_some() {
                            match whatever.as_ref().unwrap().chars().next() {
                                Some('_') => {
                                    proto = service_name.clone();
                                    tag = None;
                                    service_name = whatever.clone();
                                }
                                _ => {
                                    tag = whatever.clone();
                                    proto = service_name;
                                    service_name = None;
                                }
                            };
                        }
                        return Ok((
                            ServiceQueryType::RFC2782,
                            ServiceQueryData {
                                service: service_name,
                                tag: tag,
                                protocol: proto,
                            },
                        ));
                    }
                    ServiceQueryType::Tagged => {
                        return Ok((
                            ServiceQueryType::Tagged,
                            ServiceQueryData {
                                service: service_name,
                                tag: whatever,
                                protocol: None,
                            },
                        ))
                    }
                }
            }
        }

        return Err(anyhow!("Query is not a Service query."));
    }

    /// Returns instances
    pub async fn get_instances(&mut self) -> Result<Vec<Instance>, anyhow::Error> {
        let container_name = self.get_container_name().replace("\\", "");

        match container_name.as_str() {
            // Service queries support both container level filtering, and LXD level filtering
            "service" => {
                let instance_list = self.get_instance_list().await?;
                let mut instances: Vec<Instance> = Vec::<Instance>::new();

                for i in instance_list {
                    let metadata = match self.get_cache().await?.get(&i) {
                        Some(data) => match serde_json::from_str::<Metadaum>(&data) {
                            Ok(r) => r,
                            Err(_) => continue,
                        },
                        None => continue,
                    };

                    if metadata.status == "Running" && metadata.status_code == 103 {
                        let instance = Instance {
                            name: i,
                            data: metadata,
                        };
                        instances.push(instance);
                    }
                }

                return Ok(instances);
            }
            name => {
                let pattern = match glob::Pattern::new(name) {
                    Ok(p) => p,
                    Err(_) => return Err(anyhow!("Invalid container name.")),
                };

                let instance_list = self.get_instance_list().await?;

                let mut instances: Vec<Instance> = Vec::<Instance>::new();
                for i in instance_list {
                    if pattern.matches(&i) {
                        let metadata = match self.get_cache().await?.get(&i) {
                            Some(data) => match serde_json::from_str::<Metadaum>(&data) {
                                Ok(r) => r,
                                Err(_) => continue,
                            },
                            None => continue,
                        };

                        if metadata.status == "Running" && metadata.status_code == 103 {
                            let instance = Instance {
                                name: i,
                                data: metadata,
                            };
                            instances.push(instance);
                        }
                    }
                }

                return Ok(instances);
            }
        }
    }

    /// Returns the selected interface name if this is an interface query
    fn get_interface_name(&mut self) -> Option<String> {
        if self.get_query_type() == QueryType::Interface {
            let parts = self.get_query_pieces();
            return Some(parts[0].clone());
        }
        return None;
    }

    /// Returns matching instances
    async fn get_instance_list(&self) -> Result<Vec<String>, anyhow::Error> {
        match crate::data::get_containers(self.config.clone()).await {
            Ok(data) => data,
            Err(_) => return Err(anyhow!("Could not retrieve instance list.")),
        };

        let instance_list: Vec<String> = match self.get_cache().await?.get("instances") {
            Some(data) => match serde_json::from_str::<Vec<String>>(data.as_str()) {
                Ok(data) => data,
                Err(_) => return Err(anyhow!("Could not deserialize instance list")),
            },
            None => return Err(anyhow!("Instance list is empty.")),
        };

        return Ok(instance_list);
    }

    /// Returns the container name, which is the last element of the Vec
    pub fn get_container_name(&mut self) -> String {
        let vec = self.get_query_pieces();
        return vec[vec.len() - 1].clone();
    }

    /// Returns the query type
    pub fn get_query_type(&mut self) -> QueryType {
        let vec = self.get_query_pieces();
        if vec.len() == 1 {
            return QueryType::Container;
        }

        if vec[vec.len() - 1].as_str() == "service" {
            return QueryType::Service;
        }

        match vec[vec.len() - 2].as_str() {
            "cluster" => QueryType::Cluster,
            "if" => QueryType::Interface,
            "interface" => {
                tracing::warn!("`interface` schema is depricated, and will be removed in a future version. Migrate your client to use `.if.` instead.");
                QueryType::Interface
            }
            "service" => QueryType::Service,
            _ => QueryType::Container,
        }
    }

    /// Returns the query pieces from the local cache
    pub fn get_query_pieces(&mut self) -> Vec<String> {
        if self.query_pieces.is_none() {
            let qt = self.name.split(".").map(|s| s.to_string()).collect();
            self.query_pieces = Some(qt);
        }

        return self.query_pieces.clone().unwrap();
    }
}

impl Instance {
    /// Returns DNS RData Records for the instance
    pub fn get_records(
        &self,
        suffix: &str,
        q_type: RecordType,
        interface: Option<String>,
        use_original_name: bool,
        name_overwrite: Option<Name>,
    ) -> Vec<Record> {
        let records = self.get_all_records();

        let mut result = Vec::<Record>::new();
        for ((ttype, ifname), rdata) in records {
            if ttype.eq(&q_type) {
                match &interface {
                    Some(ifq) => {
                        if ifq == &ifname {
                            let n: Name;
                            let n: Name = match &name_overwrite {
                                Some(name) => name.clone().to_owned(),
                                None => {
                                    if use_original_name {
                                        Name::from_str(
                                            format!("{}.{}.", self.name, suffix).as_str(),
                                        )
                                        .unwrap()
                                    } else {
                                        Name::from_str(
                                            format!("{}.if.{}.{}.", ifq, self.name, suffix)
                                                .as_str(),
                                        )
                                        .unwrap()
                                    }
                                }
                            };

                            result.push(Record::from_rdata(n.clone(), 3, rdata));
                        }
                    }
                    None => {
                        // Return the container name
                        let n =
                            Name::from_str(format!("{}.{}.", self.name, suffix).as_str()).unwrap();
                        result.push(Record::from_rdata(n.clone(), 3, rdata))
                    }
                }
            }
        }

        return result;
    }

    /// Returns all records for this element
    fn get_all_records(&self) -> HashMap<(RecordType, String), RData> {
        let mut records = HashMap::<(RecordType, String), RData>::new();

        let network = self.data.state.network.clone().unwrap();
        for (ifname, network) in &network {
            for address in &network.addresses {
                if address.scope != "local" {
                    match address.family {
                        NetworkFamily::INet6 => records.insert(
                            (RecordType::AAAA, ifname.to_owned()),
                            RData::AAAA(address.address.parse().unwrap()),
                        ),
                        NetworkFamily::Inet => records.insert(
                            (RecordType::A, ifname.to_owned()),
                            RData::A(address.address.parse().unwrap()),
                        ),
                    };
                }
            }
        }

        return records;
    }

    /// Returns the services associated with this instance
    pub fn get_service_config(&self) -> Option<Vec<InstanceService>> {
        match self.data.config.get("user.service") {
            Some(data) => match serde_json::from_str::<Vec<InstanceService>>(data) {
                Ok(data) => Some(data),
                Err(error) => {
                    tracing::error!("{}", error.to_string());
                    None
                }
            },
            None => None,
        }
    }

    /// Returns the interface names available on the instance
    pub fn get_interfaces(&self) -> Vec<String> {
        let mut interfaces = Vec::<String>::new();
        let network = self.data.state.network.clone().unwrap();
        for (ifname, _) in &network {
            interfaces.push(ifname.clone().to_string());
        }

        return interfaces;
    }
}
