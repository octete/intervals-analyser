mod analysis;
mod api;
mod cli;
mod config;
mod models;
mod output;

use anyhow::{bail, Context, Result};
use chrono::NaiveDate;
use clap::Parser;

use crate::{
    analysis::{analyse_streams, AnalysisOptions, CrankSetup},
    api::IntervalsClient,
    cli::{AnalyseArgs, Cli, Command},
    config::Config,
    models::Activity,
};

#[tokio::main]
async fn main() -> Result<()> {
    let cli = Cli::parse();
    let config = Config::load(cli.config.as_deref())?;
    let client = IntervalsClient::new(config)?;

    match cli.command {
        Command::Activities(args) => {
            let activities = latest_cycling_activities(&client, args.last).await?;
            output::print_activities(&activities);
        }
        Command::Analyse(args) => run_analysis(&client, args).await?,
    }

    Ok(())
}

async fn run_analysis(client: &IntervalsClient, args: AnalyseArgs) -> Result<()> {
    validate_analyse_args(&args)?;

    let selected_activities = if let Some(from) = args.from {
        let mut activities = client.list_activities(from, args.to, None).await?;
        activities.retain(Activity::is_cycling);
        activities
    } else {
        latest_cycling_activities(client, 1).await?
    };

    if selected_activities.is_empty() {
        bail!("no cycling activities found for the requested period");
    }

    let options = AnalysisOptions {
        bucket_size_w: args.bucket_size,
        min_samples: args.min_samples,
        min_duration_secs: args.min_duration,
        crank_setup: CrankSetup {
            left_mm: args.left_crank,
            right_mm: args.right_crank,
            configured_mm: args.configured_crank,
        },
    };
    options.validate()?;

    let mut activities = Vec::with_capacity(selected_activities.len());
    let mut activity_streams = Vec::with_capacity(selected_activities.len());
    for activity in selected_activities {
        let streams = client
            .activity_streams(&activity.id)
            .await
            .with_context(|| format!("failed to fetch streams for {}", activity.id))?;
        if !has_required_streams(&streams) {
            eprintln!(
                "Skipping {} ({}): required time, watts, and left_right_balance streams are not all present.",
                activity.id, activity.start_date_local
            );
            continue;
        }
        activities.push(activity);
        activity_streams.push(streams);
    }

    if activities.is_empty() {
        bail!("none of the selected cycling activities contain power and L/R balance streams");
    }

    let report = analyse_streams(&activity_streams, &options)?;
    output::print_analysis(&activities, &report, &options);
    Ok(())
}

fn has_required_streams(streams: &[crate::models::ActivityStream]) -> bool {
    ["time", "watts", "left_right_balance"]
        .iter()
        .all(|required| streams.iter().any(|stream| stream.stream_type == *required))
}

fn validate_analyse_args(args: &AnalyseArgs) -> Result<()> {
    if args.last && args.from.is_some() {
        bail!("--last cannot be combined with --from");
    }
    if args.to.is_some() && args.from.is_none() {
        bail!("--to requires --from");
    }
    if let (Some(from), Some(to)) = (args.from, args.to) {
        if from > to {
            bail!("--from must not be later than --to");
        }
    }
    Ok(())
}

async fn latest_cycling_activities(
    client: &IntervalsClient,
    count: usize,
) -> Result<Vec<Activity>> {
    if count == 0 {
        bail!("--last must be at least 1");
    }

    // The API has no sport filter. Fetch a generous descending window, then
    // retain rides locally so a recent run does not hide the latest ride.
    let fetch_limit = count.saturating_mul(10).max(100);
    let oldest = NaiveDate::from_ymd_opt(1970, 1, 1).expect("valid fixed date");
    let mut activities = client
        .list_activities(oldest, None, Some(fetch_limit))
        .await?;
    activities.retain(Activity::is_cycling);
    activities.truncate(count);
    Ok(activities)
}

