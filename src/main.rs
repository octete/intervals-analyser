mod analysis;
mod api;
mod cli;
mod config;
mod heatmap;
mod models;
mod output;

use anyhow::{bail, Context, Result};
use chrono::NaiveDate;
use clap::Parser;

use crate::{
    analysis::{analyse_streams, AnalysisOptions, CrankSetup},
    api::IntervalsClient,
    cli::{ActivitySelectionArgs, AnalyseArgs, Cli, Command, HeatmapArgs},
    config::Config,
    heatmap::{build_heatmap, HeatmapOptions},
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
        Command::Heatmap(args) => run_heatmap(&client, args).await?,
    }

    Ok(())
}

async fn run_analysis(client: &IntervalsClient, args: AnalyseArgs) -> Result<()> {
    validate_selection(&args.selection)?;
    let selected_activities = select_activities(client, &args.selection).await?;

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

    let required = ["time", "watts", "left_right_balance"];
    let (activities, activity_streams) =
        fetch_usable_streams(client, selected_activities, &required).await?;

    let report = analyse_streams(&activity_streams, &options)?;
    output::print_analysis(&activities, &report, &options);
    Ok(())
}

async fn run_heatmap(client: &IntervalsClient, args: HeatmapArgs) -> Result<()> {
    validate_selection(&args.selection)?;
    let selected_activities = select_activities(client, &args.selection).await?;
    let options = HeatmapOptions {
        power_bucket_size_w: args.power_bucket_size,
        rpm_bucket_size: args.rpm_bucket_size,
        min_samples: args.min_samples,
        min_duration_secs: args.min_duration,
        crank_setup: CrankSetup {
            left_mm: args.left_crank,
            right_mm: args.right_crank,
            configured_mm: args.configured_crank,
        },
        balance_mode: args.balance,
    };
    options.validate()?;

    let required = ["time", "watts", "cadence", "left_right_balance"];
    let (activities, activity_streams) =
        fetch_usable_streams(client, selected_activities, &required).await?;
    let report = build_heatmap(&activity_streams, &options)?;
    output::print_heatmap(&activities, &report, &options, args.no_color);
    Ok(())
}

async fn select_activities(
    client: &IntervalsClient,
    selection: &ActivitySelectionArgs,
) -> Result<Vec<Activity>> {
    let activities = if let Some(activity_id) = selection.activity_id.as_deref() {
        let activity = client.activity(activity_id).await?;
        if !activity.is_cycling() {
            bail!(
                "activity {} is not a cycling activity (type: {})",
                activity.id,
                activity.activity_type
            );
        }
        vec![activity]
    } else if let Some(from) = selection.from {
        let mut activities = client.list_activities(from, selection.to, None).await?;
        activities.retain(Activity::is_cycling);
        activities
    } else {
        latest_cycling_activities(client, 1).await?
    };

    if activities.is_empty() {
        bail!("no cycling activities found for the requested period");
    }
    Ok(activities)
}

async fn fetch_usable_streams(
    client: &IntervalsClient,
    selected_activities: Vec<Activity>,
    required: &[&str],
) -> Result<(Vec<Activity>, Vec<Vec<crate::models::ActivityStream>>)> {
    let mut activities = Vec::with_capacity(selected_activities.len());
    let mut activity_streams = Vec::with_capacity(selected_activities.len());
    for activity in selected_activities {
        let streams = client
            .activity_streams(&activity.id)
            .await
            .with_context(|| format!("failed to fetch streams for {}", activity.id))?;
        if !has_required_streams(&streams, required) {
            eprintln!(
                "Skipping {} ({}): required streams are not all present ({}).",
                activity.id,
                activity.start_date_local,
                required.join(", ")
            );
            continue;
        }
        activities.push(activity);
        activity_streams.push(streams);
    }
    if activities.is_empty() {
        bail!("none of the selected cycling activities contain all required streams");
    }
    Ok((activities, activity_streams))
}

fn has_required_streams(streams: &[crate::models::ActivityStream], required: &[&str]) -> bool {
    required
        .iter()
        .all(|required| streams.iter().any(|stream| stream.stream_type == *required))
}

fn validate_selection(selection: &ActivitySelectionArgs) -> Result<()> {
    if selection.activity_id.as_ref().is_some_and(String::is_empty) {
        bail!("--activity-id must not be empty");
    }
    if selection.last && selection.from.is_some() {
        bail!("--last cannot be combined with --from");
    }
    if selection.to.is_some() && selection.from.is_none() {
        bail!("--to requires --from");
    }
    if let (Some(from), Some(to)) = (selection.from, selection.to) {
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
