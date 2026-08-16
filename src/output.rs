use std::io::{self, IsTerminal};

use crate::{
    analysis::{AnalysisOptions, AnalysisReport},
    cli::BalanceMode,
    heatmap::{HeatmapOptions, HeatmapReport},
    models::Activity,
};

pub fn print_activities(activities: &[Activity]) {
    println!(
        "{:<12}  {:<19}  {:>8}  {:>7}  {:>9}  NAME",
        "ID", "LOCAL START", "DURATION", "AVG W", "AVG L/R"
    );
    for activity in activities {
        let duration = activity
            .moving_time
            .map(format_duration)
            .unwrap_or_else(|| "-".to_owned());
        let watts = activity
            .icu_average_watts
            .map(|value| format!("{value:.0}"))
            .unwrap_or_else(|| "-".to_owned());
        // Activity summary avg_lr_balance follows the same right-side convention.
        let balance = activity
            .avg_lr_balance
            .map(|right| format!("{:.1}/{right:.1}", 100.0 - right))
            .unwrap_or_else(|| "-".to_owned());
        println!(
            "{:<12}  {:<19}  {:>8}  {:>7}  {:>9}  {}",
            activity.id, activity.start_date_local, duration, watts, balance, activity.name
        );
    }
}

pub fn print_analysis(activities: &[Activity], report: &AnalysisReport, options: &AnalysisOptions) {
    println!(
        "Analysed {} cycling activit{}; {} valid samples representing {}.",
        activities.len(),
        if activities.len() == 1 { "y" } else { "ies" },
        report.accepted_samples,
        format_duration(report.represented_secs.round() as u64)
    );
    println!(
        "Cranks: left {:.1} mm, right {:.1} mm, meter setting {:.1} mm. Buckets use reported watts.",
        options.crank_setup.left_mm,
        options.crank_setup.right_mm,
        options.crank_setup.configured_mm
    );
    if report.skipped_samples > 0 {
        println!(
            "Skipped {} samples with missing or invalid power/balance data.",
            report.skipped_samples
        );
    }
    if report.suppressed_buckets > 0 {
        println!(
            "Suppressed {} buckets below the thresholds.",
            report.suppressed_buckets
        );
    }

    println!();
    println!(
        "{:>13} {:>8} {:>9} {:>8} {:>8}  {:>26}  {:>26}",
        "BUCKET W",
        "SAMPLES",
        "TIME",
        "MEAN W",
        "CORR W",
        "REPORTED L/R (MEAN MED SD)",
        "CORRECTED L/R (MEAN MED SD)"
    );
    for bucket in &report.buckets {
        println!(
            "{:>6.0}-{:<6.0} {:>8} {:>9} {:>8.1} {:>8.1}  {:>5.1}/{:<5.1} {:>5.1}/{:<5.1} {:>5.2}  {:>5.1}/{:<5.1} {:>5.1}/{:<5.1} {:>5.2}",
            bucket.lower_w,
            bucket.upper_w,
            bucket.sample_count,
            format_duration(bucket.duration_secs.round() as u64),
            bucket.mean_power_w,
            bucket.mean_corrected_power_w,
            bucket.mean_reported_left_pct,
            100.0 - bucket.mean_reported_left_pct,
            bucket.median_reported_left_pct,
            100.0 - bucket.median_reported_left_pct,
            bucket.stddev_reported_left_pct,
            bucket.mean_corrected_left_pct,
            100.0 - bucket.mean_corrected_left_pct,
            bucket.median_corrected_left_pct,
            100.0 - bucket.median_corrected_left_pct,
            bucket.stddev_corrected_left_pct,
        );
    }
}

pub fn print_heatmap(
    activities: &[Activity],
    report: &HeatmapReport,
    options: &HeatmapOptions,
    no_color: bool,
) {
    let balance_label = match options.balance_mode {
        BalanceMode::Reported => "reported",
        BalanceMode::Corrected => "crank-corrected",
    };
    println!(
        "Heat map for {} cycling activit{}; {} valid samples representing {}.",
        activities.len(),
        if activities.len() == 1 { "y" } else { "ies" },
        report.accepted_samples,
        format_duration(report.represented_secs.round() as u64)
    );
    println!(
        "Cells show mean left-side {} balance; rows are {:.1} W buckets and columns are {:.1} rpm buckets.",
        balance_label, options.power_bucket_size_w, options.rpm_bucket_size
    );
    if report.skipped_samples > 0 {
        println!(
            "Skipped {} samples with missing or invalid power, cadence, or balance data.",
            report.skipped_samples
        );
    }
    if report.suppressed_cells > 0 {
        println!(
            "Suppressed {} cells below the thresholds.",
            report.suppressed_cells
        );
    }
    if report.cells.is_empty() {
        println!("No heat-map cells remain after filtering.");
        return;
    }

    let min_power = report.cells.keys().map(|(power, _)| *power).min().unwrap();
    let max_power = report.cells.keys().map(|(power, _)| *power).max().unwrap();
    let min_rpm = report.cells.keys().map(|(_, rpm)| *rpm).min().unwrap();
    let max_rpm = report.cells.keys().map(|(_, rpm)| *rpm).max().unwrap();
    let use_color = !no_color && io::stdout().is_terminal();

    println!();
    print!("{:>13}", "POWER \\ RPM");
    for rpm_bucket in min_rpm..=max_rpm {
        let lower = rpm_bucket as f64 * options.rpm_bucket_size;
        let upper = (rpm_bucket + 1) as f64 * options.rpm_bucket_size;
        print!(" {:^9}", format_range(lower, upper));
    }
    println!();

    for power_bucket in min_power..=max_power {
        let lower = power_bucket as f64 * options.power_bucket_size_w;
        let upper = (power_bucket + 1) as f64 * options.power_bucket_size_w;
        print!("{:>13}", format!("{} W", format_range(lower, upper)));
        for rpm_bucket in min_rpm..=max_rpm {
            match report.cells.get(&(power_bucket, rpm_bucket)) {
                Some(cell) => print!(" {}", heat_cell(cell.mean_left_percent, use_color)),
                None => print!(" {:^9}", "·"),
            }
        }
        println!();
    }

    println!();
    if use_color {
        println!(
            "Legend: {} left-dominant  {} near 50/50  {} right-dominant",
            heat_cell(55.0, true),
            heat_cell(50.0, true),
            heat_cell(45.0, true)
        );
    } else {
        println!(
            "Each value is mean left-leg percentage; right percentage is 100 minus the cell value."
        );
    }
}

fn heat_cell(left_percent: f64, color: bool) -> String {
    let content = format!("{:^9}", format!("{left_percent:.1}L"));
    if !color {
        return content;
    }
    let background = match left_percent - 50.0 {
        deviation if deviation >= 5.0 => 27,
        deviation if deviation >= 2.0 => 69,
        deviation if deviation > -2.0 => 240,
        deviation if deviation > -5.0 => 203,
        _ => 196,
    };
    format!("\x1b[38;5;15;48;5;{background}m{content}\x1b[0m")
}

fn format_range(lower: f64, upper: f64) -> String {
    format!("{}-{}", format_number(lower), format_number(upper))
}

fn format_number(value: f64) -> String {
    if value.fract().abs() < 1e-9 {
        format!("{value:.0}")
    } else {
        format!("{value:.1}")
    }
}

fn format_duration(seconds: u64) -> String {
    let hours = seconds / 3600;
    let minutes = (seconds % 3600) / 60;
    let seconds = seconds % 60;
    if hours > 0 {
        format!("{hours}:{minutes:02}:{seconds:02}")
    } else {
        format!("{minutes}:{seconds:02}")
    }
}
