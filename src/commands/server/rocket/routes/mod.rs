extern crate rocket;

use crate::{
    commands::server::config::ApplicationConfigLXD,
    data::{data::InstanceSimple, Query},
};
use rocket::{get, serde::json::Json, State};
#[get("/?<name>")]
pub async fn list(
    name: Option<&str>,
    lxd: &State<ApplicationConfigLXD>,
) -> Json<Vec<InstanceSimple>> {
    let n = match name {
        Some(n) => match n {
            "" => String::from("*"),
            _ => String::from(n),
        },
        None => String::from("*"),
    };

    let mut query = Query::new(n, lxd.into());

    match query.get_api_data_for_query().await {
        Ok(d) => return Json(d),
        Err(_) => return Json(Vec::<InstanceSimple>::new()),
    }
}
