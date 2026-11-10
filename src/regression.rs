use anyhow::{bail, Context, Result};

use crate::{analysis::CrankSetup, cli::BalanceMode, models::ActivityStream};

const POWER_SCALE_W: f64 = 100.0;
const CADENCE_SCALE_RPM: f64 = 10.0;

#[derive(Debug, Clone, Copy)]
pub struct RegressionOptions {
    pub min_power_w: f64,
    pub min_cadence_rpm: f64,
    pub reference_power_w: f64,
    pub reference_cadence_rpm: f64,
    pub interaction: bool,
    pub crank_setup: CrankSetup,
    pub balance_mode: BalanceMode,
}

impl RegressionOptions {
    pub fn validate(&self) -> Result<()> {
        for (label, value) in [
            ("minimum power", self.min_power_w),
            ("minimum cadence", self.min_cadence_rpm),
            ("reference power", self.reference_power_w),
            ("reference cadence", self.reference_cadence_rpm),
        ] {
            if !value.is_finite() || value < 0.0 {
                bail!("{label} must be a non-negative finite number");
            }
        }
        for (label, value) in [
            ("left crank length", self.crank_setup.left_mm),
            ("right crank length", self.crank_setup.right_mm),
            ("configured crank length", self.crank_setup.configured_mm),
        ] {
            if !value.is_finite() || value <= 0.0 {
                bail!("{label} must be a positive finite number");
            }
        }
        Ok(())
    }
}

#[derive(Debug)]
pub struct RegressionReport {
    pub intercept_pp: f64,
    pub power_coefficient_pp_per_100w: f64,
    pub cadence_coefficient_pp_per_10rpm: f64,
    pub interaction_coefficient_pp: Option<f64>,
    pub r_squared: f64,
    pub adjusted_r_squared: f64,
    pub rmse_pp: f64,
    pub mean_left_percent: f64,
    pub sample_count: usize,
    pub represented_secs: f64,
    pub invalid_samples: usize,
    pub filtered_samples: usize,
    pub min_power_w: f64,
    pub max_power_w: f64,
    pub min_cadence_rpm: f64,
    pub max_cadence_rpm: f64,
}

struct Observation {
    power_w: f64,
    cadence_rpm: f64,
    left_percent: f64,
    duration_secs: f64,
}

pub fn fit_regression(
    activity_streams: &[Vec<ActivityStream>],
    options: &RegressionOptions,
) -> Result<RegressionReport> {
    options.validate()?;
    let mut observations = Vec::new();
    let mut invalid_samples = 0;
    let mut filtered_samples = 0;

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
            let Some(power_w) = watts.get(index).copied().flatten() else {
                invalid_samples += 1;
                continue;
            };
            let Some(cadence_rpm) = cadence.get(index).copied().flatten() else {
                invalid_samples += 1;
                continue;
            };
            let Some(reported_right) = balances.get(index).copied().flatten() else {
                invalid_samples += 1;
                continue;
            };
            if !power_w.is_finite()
                || power_w < 0.0
                || !cadence_rpm.is_finite()
                || cadence_rpm < 0.0
                || !reported_right.is_finite()
                || !(0.0..=100.0).contains(&reported_right)
            {
                invalid_samples += 1;
                continue;
            }
            if power_w < options.min_power_w || cadence_rpm < options.min_cadence_rpm {
                filtered_samples += 1;
                continue;
            }

            let left_percent = match options.balance_mode {
                BalanceMode::Reported => 100.0 - reported_right,
                BalanceMode::Corrected => {
                    options
                        .crank_setup
                        .correct(power_w, reported_right)
                        .left_percent
                }
            };
            observations.push(Observation {
                power_w,
                cadence_rpm,
                left_percent,
                duration_secs: sample_duration(&times, index, fallback_delta),
            });
        }
    }

    let parameter_count = if options.interaction { 4 } else { 3 };
    if observations.len() <= parameter_count {
        bail!(
            "not enough valid samples for regression: need more than {parameter_count}, got {}",
            observations.len()
        );
    }

    let mut xtx = vec![vec![0.0; parameter_count]; parameter_count];
    let mut xty = vec![0.0; parameter_count];
    for observation in &observations {
        let predictors = predictors(observation, options);
        let outcome = observation.left_percent - 50.0;
        for row in 0..parameter_count {
            xty[row] += predictors[row] * outcome;
            for column in 0..parameter_count {
                xtx[row][column] += predictors[row] * predictors[column];
            }
        }
    }
    let coefficients = solve_linear_system(xtx, xty).context(
        "regression could not be fitted; power and cadence may not vary independently enough",
    )?;

    let mean_left_percent = observations
        .iter()
        .map(|observation| observation.left_percent)
        .sum::<f64>()
        / observations.len() as f64;
    let mut squared_error = 0.0;
    let mut total_squares = 0.0;
    for observation in &observations {
        let predicted_deviation = dot(&coefficients, &predictors(observation, options));
        let residual = (observation.left_percent - 50.0) - predicted_deviation;
        squared_error += residual * residual;
        total_squares += (observation.left_percent - mean_left_percent).powi(2);
    }
    let r_squared = if total_squares <= f64::EPSILON {
        if squared_error <= f64::EPSILON {
            1.0
        } else {
            0.0
        }
    } else {
        1.0 - squared_error / total_squares
    };
    let sample_count = observations.len();
    let adjusted_r_squared = 1.0
        - (1.0 - r_squared) * (sample_count - 1) as f64 / (sample_count - parameter_count) as f64;

    Ok(RegressionReport {
        intercept_pp: coefficients[0],
        power_coefficient_pp_per_100w: coefficients[1],
        cadence_coefficient_pp_per_10rpm: coefficients[2],
        interaction_coefficient_pp: if options.interaction {
            Some(coefficients[3])
        } else {
            None
        },
        r_squared,
        adjusted_r_squared,
        rmse_pp: (squared_error / sample_count as f64).sqrt(),
        mean_left_percent,
        sample_count,
        represented_secs: observations
            .iter()
            .map(|observation| observation.duration_secs)
            .sum(),
        invalid_samples,
        filtered_samples,
        min_power_w: observations
            .iter()
            .map(|observation| observation.power_w)
            .fold(f64::INFINITY, f64::min),
        max_power_w: observations
            .iter()
            .map(|observation| observation.power_w)
            .fold(f64::NEG_INFINITY, f64::max),
        min_cadence_rpm: observations
            .iter()
            .map(|observation| observation.cadence_rpm)
            .fold(f64::INFINITY, f64::min),
        max_cadence_rpm: observations
            .iter()
            .map(|observation| observation.cadence_rpm)
            .fold(f64::NEG_INFINITY, f64::max),
    })
}

fn predictors(observation: &Observation, options: &RegressionOptions) -> Vec<f64> {
    let power = (observation.power_w - options.reference_power_w) / POWER_SCALE_W;
    let cadence = (observation.cadence_rpm - options.reference_cadence_rpm) / CADENCE_SCALE_RPM;
    let mut values = vec![1.0, power, cadence];
    if options.interaction {
        values.push(power * cadence);
    }
    values
}

fn solve_linear_system(mut matrix: Vec<Vec<f64>>, mut values: Vec<f64>) -> Result<Vec<f64>> {
    let size = values.len();
    for pivot in 0..size {
        let best_row = (pivot..size)
            .max_by(|left, right| {
                matrix[*left][pivot]
                    .abs()
                    .total_cmp(&matrix[*right][pivot].abs())
            })
            .expect("pivot range is non-empty");
        matrix.swap(pivot, best_row);
        values.swap(pivot, best_row);

        let pivot_value = matrix[pivot][pivot];
        if !pivot_value.is_finite() || pivot_value.abs() < 1e-10 {
            bail!("singular regression matrix");
        }
        for value in matrix[pivot].iter_mut().skip(pivot) {
            *value /= pivot_value;
        }
        values[pivot] /= pivot_value;

        let pivot_row = matrix[pivot].clone();
        for row in 0..size {
            if row == pivot {
                continue;
            }
            let factor = matrix[row][pivot];
            for (column, value) in matrix[row].iter_mut().enumerate().skip(pivot) {
                *value -= factor * pivot_row[column];
            }
            values[row] -= factor * values[pivot];
        }
    }
    Ok(values)
}

fn dot(left: &[f64], right: &[f64]) -> f64 {
    left.iter()
        .zip(right)
        .map(|(left, right)| left * right)
        .sum()
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
    use approx::assert_abs_diff_eq;
    use serde_json::json;

    use super::*;

    fn stream(stream_type: &str, data: serde_json::Value) -> ActivityStream {
        ActivityStream {
            stream_type: stream_type.to_owned(),
            data,
        }
    }

    fn options(interaction: bool) -> RegressionOptions {
        RegressionOptions {
            min_power_w: 0.0,
            min_cadence_rpm: 0.0,
            reference_power_w: 200.0,
            reference_cadence_rpm: 85.0,
            interaction,
            crank_setup: CrankSetup {
                left_mm: 160.0,
                right_mm: 165.0,
                configured_mm: 162.5,
            },
            balance_mode: BalanceMode::Reported,
        }
    }

    #[test]
    fn recovers_known_power_and_cadence_coefficients() {
        let powers = vec![100.0, 100.0, 200.0, 200.0, 300.0, 300.0];
        let cadences = vec![70.0, 90.0, 70.0, 90.0, 70.0, 90.0];
        let right = powers
            .iter()
            .zip(&cadences)
            .map(|(power, cadence)| {
                let deviation =
                    1.2 + 0.8 * ((*power - 200.0) / 100.0) - 0.3 * ((*cadence - 85.0) / 10.0);
                50.0 - deviation
            })
            .collect::<Vec<_>>();
        let streams = vec![
            stream("time", json!([0, 1, 2, 3, 4, 5])),
            stream("watts", json!(powers)),
            stream("cadence", json!(cadences)),
            stream("left_right_balance", json!(right)),
        ];

        let report = fit_regression(&[streams], &options(false)).unwrap();

        assert_abs_diff_eq!(report.intercept_pp, 1.2, epsilon = 1e-10);
        assert_abs_diff_eq!(report.power_coefficient_pp_per_100w, 0.8, epsilon = 1e-10);
        assert_abs_diff_eq!(
            report.cadence_coefficient_pp_per_10rpm,
            -0.3,
            epsilon = 1e-10
        );
        assert_abs_diff_eq!(report.r_squared, 1.0, epsilon = 1e-10);
    }

    #[test]
    fn recovers_interaction_coefficient() {
        let mut powers = Vec::new();
        let mut cadences = Vec::new();
        let mut right = Vec::new();
        for power in [100.0, 200.0, 300.0] {
            for cadence in [70.0, 85.0, 100.0] {
                let p = (power - 200.0) / 100.0;
                let c = (cadence - 85.0) / 10.0;
                let deviation = 0.5 + 0.4 * p - 0.2 * c + 0.7 * p * c;
                powers.push(power);
                cadences.push(cadence);
                right.push(50.0 - deviation);
            }
        }
        let streams = vec![
            stream("time", json!([0, 1, 2, 3, 4, 5, 6, 7, 8])),
            stream("watts", json!(powers)),
            stream("cadence", json!(cadences)),
            stream("left_right_balance", json!(right)),
        ];

        let report = fit_regression(&[streams], &options(true)).unwrap();

        assert_abs_diff_eq!(
            report.interaction_coefficient_pp.unwrap(),
            0.7,
            epsilon = 1e-10
        );
    }
}
