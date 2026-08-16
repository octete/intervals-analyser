use std::path::PathBuf;

use chrono::NaiveDate;
use clap::{Args, Parser, Subcommand, ValueEnum};

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
    /// Show L/R balance as a power-by-cadence heat map.
    Heatmap(HeatmapArgs),
}

#[derive(Debug, Args)]
pub struct ActivitiesArgs {
    /// Number of recent cycling activities to list.
    #[arg(long, default_value_t = 20)]
    pub last: usize,
}

#[derive(Debug, Args)]
pub struct ActivitySelectionArgs {
    /// Select one cycling activity by its Intervals.icu activity ID.
    #[arg(
        long,
        value_name = "ID",
        conflicts_with_all = ["last", "from", "to"]
    )]
    pub activity_id: Option<String>,

    /// Analyse the most recent cycling activity (also the default without dates).
    #[arg(long, conflicts_with_all = ["activity_id", "from", "to"])]
    pub last: bool,

    /// First local activity date to include (YYYY-MM-DD).
    #[arg(long, value_name = "DATE", conflicts_with = "activity_id")]
    pub from: Option<NaiveDate>,

    /// Last local activity date to include, inclusive (YYYY-MM-DD).
    #[arg(
        long,
        value_name = "DATE",
        requires = "from",
        conflicts_with = "activity_id"
    )]
    pub to: Option<NaiveDate>,
}

#[derive(Debug, Args)]
pub struct AnalyseArgs {
    #[command(flatten)]
    pub selection: ActivitySelectionArgs,

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

#[derive(Debug, Args)]
pub struct HeatmapArgs {
    #[command(flatten)]
    pub selection: ActivitySelectionArgs,

    /// Width of each reported-power bucket in watts.
    #[arg(long, default_value_t = 50.0, value_name = "WATTS")]
    pub power_bucket_size: f64,

    /// Width of each cadence bucket in revolutions per minute.
    #[arg(long, default_value_t = 10.0, value_name = "RPM")]
    pub rpm_bucket_size: f64,

    /// Balance value displayed and coloured in each cell.
    #[arg(long, value_enum, default_value_t = BalanceMode::Corrected)]
    pub balance: BalanceMode,

    /// Suppress cells containing fewer samples.
    #[arg(long, default_value_t = 1)]
    pub min_samples: usize,

    /// Suppress cells representing fewer seconds.
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

    /// Disable ANSI background colours.
    #[arg(long)]
    pub no_color: bool,
}

#[derive(Debug, Clone, Copy, ValueEnum)]
pub enum BalanceMode {
    Reported,
    Corrected,
}

#[cfg(test)]
mod tests {
    use clap::Parser;

    use super::{Cli, Command};

    #[test]
    fn parses_activity_id_for_analysis() {
        let cli =
            Cli::try_parse_from(["lr-analysis", "analyse", "--activity-id", "i12345678"]).unwrap();

        let Command::Analyse(args) = cli.command else {
            panic!("expected analyse command");
        };
        assert_eq!(args.selection.activity_id.as_deref(), Some("i12345678"));
    }

    #[test]
    fn activity_id_conflicts_with_date_range() {
        let result = Cli::try_parse_from([
            "lr-analysis",
            "analyse",
            "--activity-id",
            "i12345678",
            "--from",
            "2026-01-01",
        ]);

        assert!(result.is_err());
    }

    #[test]
    fn parses_heatmap_bucket_sizes() {
        let cli = Cli::try_parse_from([
            "lr-analysis",
            "heatmap",
            "--activity-id",
            "i12345678",
            "--power-bucket-size",
            "25",
            "--rpm-bucket-size",
            "5",
        ])
        .unwrap();

        let Command::Heatmap(args) = cli.command else {
            panic!("expected heatmap command");
        };
        assert_eq!(args.power_bucket_size, 25.0);
        assert_eq!(args.rpm_bucket_size, 5.0);
    }
}
