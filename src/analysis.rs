use std::{cmp::Ordering, collections::BTreeMap};

use anyhow::{bail, Context, Result};

use crate::models::ActivityStream;

#[derive(Debug, Clone, Copy)]
pub struct CrankSetup {
    pub left_mm: f64,
    pub right_mm: f64,
    pub configured_mm: f64,
}

#[derive(Debug, Clone, Copy)]
pub struct CorrectedSample {
    pub total_power_w: f64,
    pub left_percent: f64,
}

impl CrankSetup {
    /// Estimate actual power by scaling each reported side by
    /// actual_crank_length / configured_crank_length.
    pub fn correct(&self, total_power_w: f64, reported_right_percent: f64) -> CorrectedSample {
        let right_fraction = reported_right_percent / 100.0;
        let left_fraction = 1.0 - right_fraction;
        let left_power = total_power_w * left_fraction * self.left_mm / self.configured_mm;
        let right_power = total_power_w * right_fraction * self.right_mm / self.configured_mm;
        let corrected_total = left_power + right_power;
        let corrected_left = if corrected_total > 0.0 {
            100.0 * left_power / corrected_total
        } else {
            100.0 * left_fraction
                * self.left_mm
                / (left_fraction * self.left_mm + right_fraction * self.right_mm)
        };

        CorrectedSample {
            total_power_w: corrected_total,
            left_percent: corrected_left,
        }
    }
}

#[derive(Debug, Clone, Copy)]
pub struct AnalysisOptions {
    pub bucket_size_w: f64,
    pub min_samples: usize,
    pub min_duration_secs: f64,
    pub crank_setup: CrankSetup,
}

impl AnalysisOptions {
    pub fn validate(&self) -> Result<()> {
        for (label, value) in [
            ("bucket size", self.bucket_size_w),
            ("minimum duration", self.min_duration_secs),
            ("left crank length", self.crank_setup.left_mm),
            ("right crank length", self.crank_setup.right_mm),
            ("configured crank length", self.crank_setup.configured_mm),
        ] {
            if !value.is_finite() || (label != "minimum duration" && value <= 0.0) {
                bail!("{label} must be a positive finite number");
            }
        }
        if self.min_duration_secs < 0.0 {
            bail!("minimum duration must not be negative");
        }
        Ok(())
    }
}

#[derive(Debug)]
pub struct AnalysisReport {
    pub buckets: Vec<BucketStats>,
    pub accepted_samples: usize,
    pub represented_secs: f64,
    pub skipped_samples: usize,
    pub suppressed_buckets: usize,
}

#[derive(Debug)]
pub struct BucketStats {
    pub lower_w: f64,
    pub upper_w: f64,
    pub sample_count: usize,
    pub duration_secs: f64,
    pub mean_power_w: f64,
    pub mean_corrected_power_w: f64,
    pub mean_reported_left_pct: f64,
    pub median_reported_left_pct: f64,
    pub stddev_reported_left_pct: f64,
    pub mean_corrected_left_pct: f64,
    pub median_corrected_left_pct: f64,
    pub stddev_corrected_left_pct: f64,
}

#[derive(Default)]
struct BucketAccumulator {
    duration_secs: f64,
    power: Vec<f64>,
    corrected_power: Vec<f64>,
    reported_left: Vec<f64>,
    corrected_left: Vec<f64>,
}

pub fn analyse_streams(
    activity_streams: &[Vec<ActivityStream>],
    options: &AnalysisOptions,
) -> Result<AnalysisReport> {
    options.validate()?;
    let mut buckets: BTreeMap<u64, BucketAccumulator> = BTreeMap::new();
    let mut accepted_samples = 0;
    let mut represented_secs = 0.0;
    let mut skipped_samples = 0;

    for streams in activity_streams {
        let times = find_stream(streams, "time")?.numeric_data()?;
        let watts = find_stream(streams, "watts")?.numeric_data()?;
        let balances = find_stream(streams, "left_right_balance")?.numeric_data()?;
        let sample_count = times.len().max(watts.len()).max(balances.len());
        let fallback_delta = median_positive_delta(&times).unwrap_or(1.0);

        for index in 0..sample_count {
            let Some(power) = watts.get(index).copied().flatten() else {
                skipped_samples += 1;
                continue;
            };
            let Some(reported_right) = balances.get(index).copied().flatten() else {
                skipped_samples += 1;
                continue;
            };
            if !power.is_finite()
                || power < 0.0
                || !reported_right.is_finite()
                || !(0.0..=100.0).contains(&reported_right)
            {
                skipped_samples += 1;
                continue;
            }

            let duration = sample_duration(&times, index, fallback_delta);
            let bucket_index = (power / options.bucket_size_w).floor() as u64;
            let reported_left = 100.0 - reported_right;
            let corrected = options.crank_setup.correct(power, reported_right);
            let bucket = buckets.entry(bucket_index).or_default();
            bucket.duration_secs += duration;
            bucket.power.push(power);
            bucket.corrected_power.push(corrected.total_power_w);
            bucket.reported_left.push(reported_left);
            bucket.corrected_left.push(corrected.left_percent);
            accepted_samples += 1;
            represented_secs += duration;
        }
    }

    let before_filter = buckets.len();
    let buckets = buckets
        .into_iter()
        .filter(|(_, bucket)| {
            bucket.power.len() >= options.min_samples
                && bucket.duration_secs >= options.min_duration_secs
        })
        .map(|(index, bucket)| bucket.finalize(index, options.bucket_size_w))
        .collect::<Vec<_>>();
    let suppressed_buckets = before_filter - buckets.len();

    Ok(AnalysisReport {
        buckets,
        accepted_samples,
        represented_secs,
        skipped_samples,
        suppressed_buckets,
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
    median(&mut deltas)
}

fn sample_duration(times: &[Option<f64>], index: usize, fallback: f64) -> f64 {
    match (times.get(index), times.get(index + 1)) {
        (Some(Some(first)), Some(Some(second))) if second > first && (second - first).is_finite() => {
            second - first
        }
        _ => fallback,
    }
}

impl BucketAccumulator {
    fn finalize(mut self, index: u64, bucket_size: f64) -> BucketStats {
        BucketStats {
            lower_w: index as f64 * bucket_size,
            upper_w: (index + 1) as f64 * bucket_size,
            sample_count: self.power.len(),
            duration_secs: self.duration_secs,
            mean_power_w: mean(&self.power),
            mean_corrected_power_w: mean(&self.corrected_power),
            mean_reported_left_pct: mean(&self.reported_left),
            median_reported_left_pct: median(&mut self.reported_left).unwrap_or_default(),
            stddev_reported_left_pct: population_stddev(&self.reported_left),
            mean_corrected_left_pct: mean(&self.corrected_left),
            median_corrected_left_pct: median(&mut self.corrected_left).unwrap_or_default(),
            stddev_corrected_left_pct: population_stddev(&self.corrected_left),
        }
    }
}

fn mean(values: &[f64]) -> f64 {
    values.iter().sum::<f64>() / values.len() as f64
}

fn median(values: &mut [f64]) -> Option<f64> {
    if values.is_empty() {
        return None;
    }
    values.sort_by(|left, right| left.partial_cmp(right).unwrap_or(Ordering::Equal));
    let middle = values.len() / 2;
    if values.len() % 2 == 0 {
        Some((values[middle - 1] + values[middle]) / 2.0)
    } else {
        Some(values[middle])
    }
}

fn population_stddev(values: &[f64]) -> f64 {
    let average = mean(values);
    (values
        .iter()
        .map(|value| (value - average).powi(2))
        .sum::<f64>()
        / values.len() as f64)
        .sqrt()
}

#[cfg(test)]
mod tests {
    use approx::assert_abs_diff_eq;
    use serde_json::json;

    use super::*;

    fn stream(stream_type: &str, data: serde_json::Value) -> ActivityStream {
        ActivityStream {
            stream_type: stream_type.to_owned(),
            data,
        }
    }

    fn options(bucket_size_w: f64) -> AnalysisOptions {
        AnalysisOptions {
            bucket_size_w,
            min_samples: 1,
            min_duration_secs: 0.0,
            crank_setup: CrankSetup {
                left_mm: 160.0,
                right_mm: 165.0,
                configured_mm: 162.5,
            },
        }
    }

    #[test]
    fn buckets_are_lower_inclusive_and_upper_exclusive() {
        let streams = vec![
            stream("time", json!([0, 1, 2, 3])),
            stream("watts", json!([0, 49.999, 50, 100])),
            stream("left_right_balance", json!([50, 50, 50, 50])),
        ];

        let report = analyse_streams(&[streams], &options(50.0)).unwrap();

        assert_eq!(report.buckets.len(), 3);
        assert_eq!(report.buckets[0].sample_count, 2);
        assert_eq!(report.buckets[0].lower_w, 0.0);
        assert_eq!(report.buckets[0].upper_w, 50.0);
        assert_eq!(report.buckets[1].sample_count, 1);
        assert_eq!(report.buckets[1].lower_w, 50.0);
        assert_eq!(report.buckets[2].lower_w, 100.0);
    }

    #[test]
    fn configurable_bucket_size_is_used() {
        let streams = vec![
            stream("time", json!([0, 1, 2])),
            stream("watts", json!([24.9, 25, 74.9])),
            stream("left_right_balance", json!([50, 50, 50])),
        ];

        let report = analyse_streams(&[streams], &options(25.0)).unwrap();

        assert_eq!(
            report
                .buckets
                .iter()
                .map(|bucket| bucket.lower_w)
                .collect::<Vec<_>>(),
            vec![0.0, 25.0, 50.0]
        );
    }

    #[test]
    fn crank_correction_scales_sides_and_recomputes_balance() {
        let setup = CrankSetup {
            left_mm: 160.0,
            right_mm: 165.0,
            configured_mm: 162.5,
        };

        let corrected = setup.correct(200.0, 50.0);

        assert_abs_diff_eq!(corrected.total_power_w, 200.0, epsilon = 1e-12);
        assert_abs_diff_eq!(corrected.left_percent, 160.0 / 325.0 * 100.0, epsilon = 1e-12);
    }

    #[test]
    fn crank_correction_uses_configured_length_for_total_power() {
        let setup = CrankSetup {
            left_mm: 160.0,
            right_mm: 165.0,
            configured_mm: 170.0,
        };

        let corrected = setup.correct(200.0, 50.0);

        assert_abs_diff_eq!(corrected.total_power_w, 200.0 * 162.5 / 170.0, epsilon = 1e-12);
        assert_abs_diff_eq!(corrected.left_percent, 160.0 / 325.0 * 100.0, epsilon = 1e-12);
    }

    #[test]
    fn minimum_thresholds_suppress_small_buckets() {
        let streams = vec![
            stream("time", json!([0, 1, 2])),
            stream("watts", json!([10, 60, 61])),
            stream("left_right_balance", json!([50, 50, 50])),
        ];
        let mut options = options(50.0);
        options.min_samples = 2;

        let report = analyse_streams(&[streams], &options).unwrap();

        assert_eq!(report.buckets.len(), 1);
        assert_eq!(report.buckets[0].lower_w, 50.0);
        assert_eq!(report.suppressed_buckets, 1);
    }
}

