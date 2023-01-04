extern crate rocket;

use crate::{commands::server::config::ApplicationConfigLXD, data::Query};
use rocket::{get, serde::json::Json, State};
use serde::{Deserialize, Serialize};

#[derive(Default, Debug, Clone, PartialEq, Serialize, Deserialize)]
pub struct Instance {}

#[get("/")]
pub fn list(lxd: &State<ApplicationConfigLXD>) -> Json<Vec<Instance>> {
    let instances = Vec::<Instance>::new();

    return Json(instances);
}
