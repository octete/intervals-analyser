use std::{env, fs, path::Path};

use anyhow::{bail, Context, Result};
use serde::Deserialize;

const DEFAULT_BASE_URL: &str = "https://intervals.icu";

#[derive(Clone)]
pub struct Config {
    pub api_key: String,
    pub athlete_id: String,
    pub base_url: String,
}

#[derive(Default, Deserialize)]
struct FileConfig {
    api_key: Option<String>,
    athlete_id: Option<String>,
    base_url: Option<String>,
}

impl Config {
    pub fn load(explicit_path: Option<&Path>) -> Result<Self> {
        let file_config = match explicit_path {
            Some(path) => read_config(path)?,
            None => default_config_path()
                .filter(|path| path.is_file())
                .map_or_else(|| Ok(FileConfig::default()), |path| read_config(&path))?,
        };

        let api_key = env::var("INTERVALS_API_KEY")
            .ok()
            .or(file_config.api_key)
            .filter(|value| !value.trim().is_empty())
            .context(
                "missing API key: set INTERVALS_API_KEY or api_key in the config file",
            )?;
        let athlete_id = env::var("INTERVALS_ATHLETE_ID")
            .ok()
            .or(file_config.athlete_id)
            .filter(|value| !value.trim().is_empty())
            .context(
                "missing athlete ID: set INTERVALS_ATHLETE_ID or athlete_id in the config file",
            )?;
        let base_url = env::var("INTERVALS_BASE_URL")
            .ok()
            .or(file_config.base_url)
            .unwrap_or_else(|| DEFAULT_BASE_URL.to_owned());

        validate_path_component("athlete ID", &athlete_id)?;
        if !(base_url.starts_with("https://") || base_url.starts_with("http://localhost")) {
            bail!("base_url must use HTTPS (HTTP is allowed only for localhost testing)");
        }

        Ok(Self {
            api_key,
            athlete_id,
            base_url: base_url.trim_end_matches('/').to_owned(),
        })
    }
}

fn read_config(path: &Path) -> Result<FileConfig> {
    let contents = fs::read_to_string(path)
        .with_context(|| format!("failed to read config file {}", path.display()))?;
    toml::from_str(&contents)
        .with_context(|| format!("failed to parse config file {}", path.display()))
}

fn default_config_path() -> Option<std::path::PathBuf> {
    dirs::config_dir().map(|directory| directory.join("lr-analysis").join("config.toml"))
}

fn validate_path_component(label: &str, value: &str) -> Result<()> {
    if !value
        .chars()
        .all(|character| character.is_ascii_alphanumeric() || matches!(character, '_' | '-'))
    {
        bail!("{label} contains unsupported characters");
    }
    Ok(())
}

