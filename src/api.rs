use anyhow::{bail, Context, Result};
use chrono::NaiveDate;
use reqwest::{Client, StatusCode};

use crate::{
    config::Config,
    models::{Activity, ActivityStream},
};

pub struct IntervalsClient {
    http: Client,
    config: Config,
}

impl IntervalsClient {
    pub fn new(config: Config) -> Result<Self> {
        let http = Client::builder()
            .user_agent(concat!("lr-analysis/", env!("CARGO_PKG_VERSION")))
            .build()
            .context("failed to construct HTTP client")?;
        Ok(Self { http, config })
    }

    pub async fn list_activities(
        &self,
        oldest: NaiveDate,
        newest: Option<NaiveDate>,
        limit: Option<usize>,
    ) -> Result<Vec<Activity>> {
        let url = format!(
            "{}/api/v1/athlete/{}/activities",
            self.config.base_url, self.config.athlete_id
        );
        let mut query = vec![("oldest", oldest.to_string())];
        if let Some(newest) = newest {
            query.push(("newest", newest.to_string()));
        }
        if let Some(limit) = limit {
            query.push(("limit", limit.to_string()));
        }

        let response = self
            .http
            .get(url)
            .basic_auth("API_KEY", Some(&self.config.api_key))
            .query(&query)
            .send()
            .await
            .context("failed to contact Intervals.icu")?;
        parse_response(response, "list activities").await
    }

    pub async fn activity(&self, activity_id: &str) -> Result<Activity> {
        validate_activity_id(activity_id)?;
        let url = format!("{}/api/v1/activity/{activity_id}", self.config.base_url);
        let response = self
            .http
            .get(url)
            .basic_auth("API_KEY", Some(&self.config.api_key))
            .send()
            .await
            .context("failed to contact Intervals.icu")?;
        parse_response(response, "fetch activity").await
    }

    pub async fn activity_streams(&self, activity_id: &str) -> Result<Vec<ActivityStream>> {
        validate_activity_id(activity_id)?;
        // The OpenAPI path parameter `ext` is required. `.json` selects JSON;
        // asking for watts specifically maps Intervals.icu's fixed_watts to watts.
        let url = format!(
            "{}/api/v1/activity/{activity_id}/streams.json",
            self.config.base_url
        );
        let response = self
            .http
            .get(url)
            .basic_auth("API_KEY", Some(&self.config.api_key))
            .query(&[("types", "time,watts,cadence,left_right_balance")])
            .send()
            .await
            .context("failed to contact Intervals.icu")?;
        parse_response(response, "fetch activity streams").await
    }
}

async fn parse_response<T: serde::de::DeserializeOwned>(
    response: reqwest::Response,
    action: &str,
) -> Result<T> {
    let status = response.status();
    if status.is_success() {
        return response.json().await.with_context(|| {
            format!("Intervals.icu returned invalid JSON while trying to {action}")
        });
    }

    let retry_after = response
        .headers()
        .get(reqwest::header::RETRY_AFTER)
        .and_then(|value| value.to_str().ok())
        .map(str::to_owned);
    let body = response.text().await.unwrap_or_default();
    let body = body.trim().chars().take(300).collect::<String>();
    if status == StatusCode::UNAUTHORIZED || status == StatusCode::FORBIDDEN {
        bail!(
            "Intervals.icu rejected the credentials or access ({status}) while trying to {action}"
        );
    }
    if status == StatusCode::TOO_MANY_REQUESTS {
        bail!(
            "Intervals.icu rate limit reached while trying to {action}{}",
            retry_after
                .map(|seconds| format!("; retry after {seconds} seconds"))
                .unwrap_or_default()
        );
    }
    bail!("Intervals.icu error {status} while trying to {action}: {body}")
}

fn validate_activity_id(value: &str) -> Result<()> {
    if !value
        .chars()
        .all(|character| character.is_ascii_alphanumeric() || matches!(character, '_' | '-'))
    {
        bail!("activity ID contains unsupported characters");
    }
    Ok(())
}
