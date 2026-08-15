use anyhow::{bail, Context, Result};
use serde::Deserialize;
use serde_json::Value;

#[derive(Debug, Clone, Deserialize)]
pub struct Activity {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub name: String,
    #[serde(default)]
    pub start_date_local: String,
    #[serde(rename = "type", default)]
    pub activity_type: String,
    #[serde(default)]
    pub moving_time: Option<u64>,
    #[serde(default)]
    pub icu_average_watts: Option<f64>,
    #[serde(default)]
    pub avg_lr_balance: Option<f64>,
}

impl Activity {
    pub fn is_cycling(&self) -> bool {
        let activity_type = self.activity_type.to_ascii_lowercase();
        activity_type.contains("ride")
            || activity_type.contains("cycling")
            || matches!(activity_type.as_str(), "handcycle" | "velomobile")
    }
}

#[derive(Debug, Clone, Deserialize)]
pub struct ActivityStream {
    #[serde(rename = "type")]
    pub stream_type: String,
    pub data: Value,
}

impl ActivityStream {
    pub fn numeric_data(&self) -> Result<Vec<Option<f64>>> {
        let values = self
            .data
            .as_array()
            .with_context(|| format!("stream '{}' data is not an array", self.stream_type))?;

        values
            .iter()
            .enumerate()
            .map(|(index, value)| match value {
                Value::Null => Ok(None),
                Value::Number(number) => number.as_f64().map(Some).with_context(|| {
                    format!("stream '{}' value {index} is not finite", self.stream_type)
                }),
                _ => bail!(
                    "stream '{}' value {index} is not numeric or null",
                    self.stream_type
                ),
            })
            .collect()
    }
}
