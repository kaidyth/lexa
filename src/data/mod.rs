use crate::commands::server::config::ApplicationConfigLXD;
use anyhow::anyhow;

pub(crate) mod containers_full;
mod data;
pub(crate) use data::Query;

pub(crate) async fn get_containers(
    config: ApplicationConfigLXD,
) -> Result<containers_full::Root, anyhow::Error> {
    let cache = match crate::CACHE.get() {
        Some(cache) => match cache {
            Some(cache) => cache,
            None => return Err(anyhow!("Unable to retrieve cache.")),
        },
        None => return Err(anyhow!("Unable to retrieve cache.")),
    };

    match cache.get("containers_full") {
        Some(data) => {
            tracing::debug!("Retrieved data from Cache");
            return Ok(serde_json::from_str::<containers_full::Root>(&data).unwrap());
        }
        None => {
            tracing::debug!("Data not present in cache.");
            let client = config.get_client()?;

            match client
                .get(format!(
                    "https://{}:{}/1.0/containers?recursion=2",
                    config.get_fqdn(), config.bind.port
                ))
                .send()
                .await
            {
                Ok(response) => match response.json().await {
                    Ok(json) => match serde_json::from_value::<containers_full::Root>(json) {
                        Ok(data) => {
                            tracing::debug!("Retrieved Data from API");
                            cache
                                .insert(
                                    "containers_full".to_string(),
                                    serde_json::to_string(&data.clone()).unwrap(),
                                )
                                .await;

                            let mut instances: Vec<String> = Vec::<String>::new();
                            for instance in data.clone().metadata {
                                instances.push(instance.name.clone());
                                cache
                                    .insert(
                                        instance.name.clone(),
                                        serde_json::to_string(&instance).unwrap(),
                                    )
                                    .await;
                            }

                            cache
                                .insert(
                                    "instances".to_string(),
                                    serde_json::to_string(&instances).unwrap(),
                                )
                                .await;
                            return Ok(data);
                        }
                        Err(error) => {
                            tracing::error!("{}", error.to_string());
                            return Err(anyhow!("Unable to connect to LXD"));
                        }
                    },
                    Err(error) => {
                        tracing::error!("{}", error.to_string());
                        return Err(anyhow!("Could not parse LXD 1.0 Response"));
                    }
                },
                Err(error) => {
                    tracing::error!("{}", error.to_string());
                    return Err(anyhow!("Unable to connect to LXD"));
                }
            }
        }
    };
}
