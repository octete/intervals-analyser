use crate::{
    analysis::{AnalysisOptions, AnalysisReport},
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
            activity.id,
            activity.start_date_local,
            duration,
            watts,
            balance,
            activity.name
        );
    }
}

pub fn print_analysis(
    activities: &[Activity],
    report: &AnalysisReport,
    options: &AnalysisOptions,
) {
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
        println!("Suppressed {} buckets below the thresholds.", report.suppressed_buckets);
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
