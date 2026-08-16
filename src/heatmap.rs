use std::collections::BTreeMap;

use anyhow::{bail, Context, Result};

use crate::{analysis::CrankSetup, cli::BalanceMode, models::ActivityStream};

#[derive(Debug, Clone, Copy)]
pub struct HeatmapOptions {
    pub power_bucket_size_w: f64,
    pub rpm_bucket_size: f64,
    pub min_samples: usize,
    pub min_duration_secs: f64,
    pub crank_setup: CrankSetup,
    pub balance_mode: BalanceMode,
}

impl HeatmapOptions {
    pub fn validate(&self) -> Result<()> {
        for (label, value) in [
            ("power bucket size", self.power_bucket_size_w),
            ("rpm bucket size", self.rpm_bucket_size),
            ("left crank length", self.crank_setup.left_mm),
            ("right crank length", self.crank_setup.right_mm),
            ("configured crank length", self.crank_setup.configured_mm),
        ] {
            if !value.is_finite() || value <= 0.0 {
                bail!("{label} must be a positive finite number");
            }
        }
        if !self.min_duration_secs.is_finite() || self.min_duration_secs < 0.0 {
            bail!("minimum duration must be a non-negative finite number");
        }
        Ok(())
    }
}

#[derive(Debug)]
pub struct HeatmapReport {
    pub cells: BTreeMap<(u64, u64), HeatmapCell>,
    pub accepted_samples: usize,
    pub represented_secs: f64,
    pub skipped_samples: usize,
    pub suppressed_cells: usize,
}

#[derive(Debug)]
pub struct HeatmapCell {
    pub mean_left_percent: f64,
}

#[derive(Default)]
struct CellAccumulator {
    sample_count: usize,
    duration_secs: f64,
    left_percent_sum: f64,
}

pub fn build_heatmap(
    activity_streams: &[Vec<ActivityStream>],
    options: &HeatmapOptions,
) -> Result<HeatmapReport> {
    options.validate()?;
    let mut cells: BTreeMap<(u64, u64), CellAccumulator> = BTreeMap::new();
    let mut accepted_samples = 0;
    let mut represented_secs = 0.0;
    let mut skipped_samples = 0;

    for streams in activity_streams {
        let times = find_stream(streams, "time")?.numeric_data()?;
        let watts = find_stream(streams, "watts")?.numeric_data()?;
        let cadence = find_stream(streams, "cadence")?.numeric_data()?;
        let balances = find_stream(streams, "left_right_balance")?.numeric_data()?;
        let sample_count = times
            .len()
            .max(watts.len())
            .max(cadence.len())
            .max(balances.len());
        let fallback_delta = median_positive_delta(&times).unwrap_or(1.0);

        for index in 0..sample_count {
            let Some(power) = watts.get(index).copied().flatten() else {
                skipped_samples += 1;
                continue;
            };
            let Some(rpm) = cadence.get(index).copied().flatten() else {
                skipped_samples += 1;
                continue;
            };
            let Some(reported_right) = balances.get(index).copied().flatten() else {
                skipped_samples += 1;
                continue;
            };
            if !power.is_finite()
                || power < 0.0
                || !rpm.is_finite()
                || rpm < 0.0
                || !reported_right.is_finite()
                || !(0.0..=100.0).contains(&reported_right)
            {
                skipped_samples += 1;
                continue;
            }

            let left_percent = match options.balance_mode {
                BalanceMode::Reported => 100.0 - reported_right,
                BalanceMode::Corrected => {
                    options
                        .crank_setup
                        .correct(power, reported_right)
                        .left_percent
                }
            };
            let duration = sample_duration(&times, index, fallback_delta);
            let power_bucket = (power / options.power_bucket_size_w).floor() as u64;
            let rpm_bucket = (rpm / options.rpm_bucket_size).floor() as u64;
            let cell = cells.entry((power_bucket, rpm_bucket)).or_default();
            cell.sample_count += 1;
            cell.duration_secs += duration;
            cell.left_percent_sum += left_percent;
            accepted_samples += 1;
            represented_secs += duration;
        }
    }

    let before_filter = cells.len();
    let cells = cells
        .into_iter()
        .filter(|(_, cell)| {
            cell.sample_count >= options.min_samples
                && cell.duration_secs >= options.min_duration_secs
        })
        .map(|(key, cell)| {
            (
                key,
                HeatmapCell {
                    mean_left_percent: cell.left_percent_sum / cell.sample_count as f64,
                },
            )
        })
        .collect::<BTreeMap<_, _>>();
    let suppressed_cells = before_filter - cells.len();

    Ok(HeatmapReport {
        cells,
        accepted_samples,
        represented_secs,
        skipped_samples,
        suppressed_cells,
    })
}

fn find_stream<'a>(streams: &'a [ActivityStream], stream_type: &str) -> Result<&'a ActivityStream> {
    streams
        .iter()
        .find(|stream| stream.stream_type == stream_type)
        .with_context(|| format!("activity does not contain a '{stream_type}' stream"))
}

fn median_positive_delta(times: &[Option<f64>]) -> Option<f64> {
    let mut deltas = times
        .windows(2)
        .filter_map(|pair| match pair {
            [Some(first), Some(second)] if second > first => Some(second - first),
            _ => None,
        })
        .filter(|delta| delta.is_finite())
        .collect::<Vec<_>>();
    if deltas.is_empty() {
        return None;
    }
    deltas.sort_by(f64::total_cmp);
    let middle = deltas.len() / 2;
    Some(if deltas.len().is_multiple_of(2) {
        (deltas[middle - 1] + deltas[middle]) / 2.0
    } else {
        deltas[middle]
    })
}

fn sample_duration(times: &[Option<f64>], index: usize, fallback: f64) -> f64 {
    match (times.get(index), times.get(index + 1)) {
        (Some(Some(first)), Some(Some(second)))
            if second > first && (second - first).is_finite() =>
        {
            second - first
        }
        _ => fallback,
    }
}

#[cfg(test)]
mod tests {
    use serde_json::json;

    use super::*;

    fn stream(stream_type: &str, data: serde_json::Value) -> ActivityStream {
        ActivityStream {
            stream_type: stream_type.to_owned(),
            data,
        }
    }

    fn options() -> HeatmapOptions {
        HeatmapOptions {
            power_bucket_size_w: 50.0,
            rpm_bucket_size: 10.0,
            min_samples: 1,
            min_duration_secs: 0.0,
            crank_setup: CrankSetup {
                left_mm: 160.0,
                right_mm: 165.0,
                configured_mm: 162.5,
            },
            balance_mode: BalanceMode::Reported,
        }
    }

    #[test]
    fn buckets_power_and_cadence_on_lower_inclusive_boundaries() {
        let streams = vec![
            stream("time", json!([0, 1, 2])),
            stream("watts", json!([49.9, 50, 100])),
            stream("cadence", json!([79.9, 80, 90])),
            stream("left_right_balance", json!([50, 51, 52])),
        ];

        let report = build_heatmap(&[streams], &options()).unwrap();

        assert!(report.cells.contains_key(&(0, 7)));
        assert!(report.cells.contains_key(&(1, 8)));
        assert!(report.cells.contains_key(&(2, 9)));
    }

    #[test]
    fn suppresses_cells_below_minimum_samples() {
        let streams = vec![
            stream("time", json!([0, 1, 2])),
            stream("watts", json!([50, 51, 100])),
            stream("cadence", json!([80, 81, 90])),
            stream("left_right_balance", json!([50, 50, 50])),
        ];
        let mut options = options();
        options.min_samples = 2;

        let report = build_heatmap(&[streams], &options).unwrap();

        assert_eq!(report.cells.len(), 1);
        assert!(report.cells.contains_key(&(1, 8)));
        assert_eq!(report.suppressed_cells, 1);
    }
}
