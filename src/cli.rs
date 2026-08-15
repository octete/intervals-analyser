use std::path::PathBuf;

use chrono::NaiveDate;
use clap::{Args, Parser, Subcommand};

#[derive(Debug, Parser)]
#[command(name = "lr-analysis", version, about)]
pub struct Cli {
    /// TOML config file. Defaults to the platform config directory.
    #[arg(long, global = true, value_name = "PATH")]
    pub config: Option<PathBuf>,

    #[command(subcommand)]
    pub command: Command,
}

#[derive(Debug, Subcommand)]
pub enum Command {
    /// List recent cycling activities.
    Activities(ActivitiesArgs),
    /// Analyse power and left/right balance.
    Analyse(AnalyseArgs),
}

#[derive(Debug, Args)]
pub struct ActivitiesArgs {
    /// Number of recent cycling activities to list.
    #[arg(long, default_value_t = 20)]
    pub last: usize,
}

#[derive(Debug, Args)]
pub struct AnalyseArgs {
    /// Analyse the most recent cycling activity (also the default without dates).
    #[arg(long, conflicts_with = "from")]
    pub last: bool,

    /// First local activity date to include (YYYY-MM-DD).
    #[arg(long, value_name = "DATE")]
    pub from: Option<NaiveDate>,

    /// Last local activity date to include, inclusive (YYYY-MM-DD).
    #[arg(long, value_name = "DATE", requires = "from")]
    pub to: Option<NaiveDate>,

    /// Width of each reported-power bucket in watts.
    #[arg(long, default_value_t = 50.0, value_name = "WATTS")]
    pub bucket_size: f64,

    /// Suppress buckets containing fewer samples.
    #[arg(long, default_value_t = 1)]
    pub min_samples: usize,

    /// Suppress buckets representing fewer seconds.
    #[arg(long, default_value_t = 0.0, value_name = "SECONDS")]
    pub min_duration: f64,

    /// Actual left crank length in millimetres.
    #[arg(long, default_value_t = 160.0, value_name = "MM")]
    pub left_crank: f64,

    /// Actual right crank length in millimetres.
    #[arg(long, default_value_t = 165.0, value_name = "MM")]
    pub right_crank: f64,

    /// Single crank length configured in the power meter.
    #[arg(long, default_value_t = 162.5, value_name = "MM")]
    pub configured_crank: f64,
}

