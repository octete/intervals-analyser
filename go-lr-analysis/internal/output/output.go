package output

import (
	"fmt"
	"math"
	"os"

	"github.com/local/lr-analysis-go/internal/model"
	"github.com/local/lr-analysis-go/internal/stats"
)

func PrintActivities(activities []model.Activity) {
	fmt.Printf("%-12s  %-19s  %8s  %7s  %9s  %s\n", "ID", "LOCAL START", "DURATION", "AVG W", "AVG L/R", "NAME")
	for _, activity := range activities {
		duration, watts, balance := "-", "-", "-"
		if activity.MovingTime != nil {
			duration = formatDuration(float64(*activity.MovingTime))
		}
		if activity.AverageWatts != nil {
			watts = fmt.Sprintf("%.0f", *activity.AverageWatts)
		}
		if activity.AverageLRBalance != nil {
			balance = fmt.Sprintf("%.1f/%.1f", 100-*activity.AverageLRBalance, *activity.AverageLRBalance)
		}
		fmt.Printf("%-12s  %-19s  %8s  %7s  %9s  %s\n", activity.ID, activity.StartDateLocal, duration, watts, balance, activity.Name)
	}
}

func PrintAnalysis(activities []model.Activity, report stats.AnalysisReport, options stats.AnalysisOptions) {
	fmt.Printf("Analysed %d cycling %s; %d valid samples representing %s.\n", len(activities), activityWord(len(activities)), report.AcceptedSamples, formatDuration(report.RepresentedSecs))
	fmt.Printf("Cranks: left %.1f mm, right %.1f mm, meter setting %.1f mm. Buckets use reported watts.\n", options.Cranks.LeftMM, options.Cranks.RightMM, options.Cranks.ConfiguredMM)
	if report.SkippedSamples > 0 {
		fmt.Printf("Skipped %d samples with missing or invalid power/balance data.\n", report.SkippedSamples)
	}
	if report.SuppressedBuckets > 0 {
		fmt.Printf("Suppressed %d buckets below the thresholds.\n", report.SuppressedBuckets)
	}
	fmt.Printf("\n%13s %8s %9s %8s %8s  %31s  %31s\n", "BUCKET W", "SAMPLES", "TIME", "MEAN W", "CORR W", "REPORTED L/R (MEAN MED SD)", "CORRECTED L/R (MEAN MED SD)")
	for _, b := range report.Buckets {
		fmt.Printf("%6.0f-%-6.0f %8d %9s %8.1f %8.1f  %5.1f/%-5.1f %5.1f/%-5.1f %5.2f  %5.1f/%-5.1f %5.1f/%-5.1f %5.2f\n", b.LowerW, b.UpperW, b.SampleCount, formatDuration(b.DurationSecs), b.MeanPowerW, b.MeanCorrectedPowerW, b.MeanReportedLeftPercent, 100-b.MeanReportedLeftPercent, b.MedianReportedLeftPercent, 100-b.MedianReportedLeftPercent, b.StdDevReportedLeftPercent, b.MeanCorrectedLeftPercent, 100-b.MeanCorrectedLeftPercent, b.MedianCorrectedLeftPercent, 100-b.MedianCorrectedLeftPercent, b.StdDevCorrectedLeftPercent)
	}
}

func PrintHeatmap(activities []model.Activity, report stats.HeatmapReport, options stats.HeatmapOptions, noColor bool) {
	fmt.Printf("Heat map for %d cycling %s; %d valid samples representing %s.\n", len(activities), activityWord(len(activities)), report.AcceptedSamples, formatDuration(report.RepresentedSecs))
	fmt.Printf("Cells show mean %s L/R balance; rows are %.1f W buckets and columns are %.1f rpm buckets.\n", options.BalanceMode, options.PowerBucketSizeW, options.RPMBucketSize)
	if report.SkippedSamples > 0 {
		fmt.Printf("Skipped %d samples with missing or invalid power, cadence, or balance data.\n", report.SkippedSamples)
	}
	if report.SuppressedCells > 0 {
		fmt.Printf("Suppressed %d cells below the thresholds.\n", report.SuppressedCells)
	}
	if len(report.Cells) == 0 {
		fmt.Println("No heat-map cells remain after filtering.")
		return
	}
	minPower, maxPower, minRPM, maxRPM := math.MaxInt, math.MinInt, math.MaxInt, math.MinInt
	for key := range report.Cells {
		if key.Power < minPower {
			minPower = key.Power
		}
		if key.Power > maxPower {
			maxPower = key.Power
		}
		if key.RPM < minRPM {
			minRPM = key.RPM
		}
		if key.RPM > maxRPM {
			maxRPM = key.RPM
		}
	}
	useColor := !noColor && isTerminal(os.Stdout)
	fmt.Printf("\n%13s", "POWER \\ RPM")
	for rpm := minRPM; rpm <= maxRPM; rpm++ {
		fmt.Printf(" %11s", formatRange(float64(rpm)*options.RPMBucketSize, float64(rpm+1)*options.RPMBucketSize))
	}
	fmt.Println()
	for power := minPower; power <= maxPower; power++ {
		fmt.Printf("%13s", formatRange(float64(power)*options.PowerBucketSizeW, float64(power+1)*options.PowerBucketSizeW)+" W")
		for rpm := minRPM; rpm <= maxRPM; rpm++ {
			if cell, ok := report.Cells[stats.CellKey{Power: power, RPM: rpm}]; ok {
				fmt.Printf(" %s", heatCell(cell.MeanLeftPercent, useColor))
			} else {
				fmt.Printf(" %11s", "·")
			}
		}
		fmt.Println()
	}
	fmt.Println()
	if useColor {
		fmt.Printf("Legend: %s left-dominant  %s near 50/50  %s right-dominant\n", heatCell(55, true), heatCell(50, true), heatCell(45, true))
	} else {
		fmt.Println("Each value is the mean L/R percentage for that power/cadence cell.")
	}
}

func PrintRegression(activities []model.Activity, report stats.RegressionReport, options stats.RegressionOptions) {
	fmt.Printf("Regression for %d cycling %s using %d %s-balance samples representing %s.\n", len(activities), activityWord(len(activities)), report.SampleCount, options.BalanceMode, formatDuration(report.RepresentedSecs))
	fmt.Printf("Included range: %.0f-%.0f W and %.0f-%.0f rpm (filters: power >= %.0f W, cadence >= %.0f rpm).\n", report.MinPowerW, report.MaxPowerW, report.MinCadenceRPM, report.MaxCadenceRPM, options.MinPowerW, options.MinCadenceRPM)
	if report.FilteredSamples > 0 || report.InvalidSamples > 0 {
		fmt.Printf("Excluded %d samples below filters and %d samples with missing or invalid data.\n", report.FilteredSamples, report.InvalidSamples)
	}
	fmt.Printf("\nL - 50 = %.4f %+.4f × ((P - %.0f) / 100) %+.4f × ((C - %.0f) / 10)", report.InterceptPP, report.PowerCoefficientPPPer100W, options.ReferencePowerW, report.CadenceCoefficientPPPer10RPM, options.ReferenceCadenceRPM)
	if report.InteractionCoefficientPP != nil {
		fmt.Printf(" %+.4f × ((P - %.0f) / 100) × ((C - %.0f) / 10)", *report.InteractionCoefficientPP, options.ReferencePowerW, options.ReferenceCadenceRPM)
	}
	fmt.Println()
	fmt.Printf("Reference prediction at %.0f W / %.0f rpm: %s L/R\n", options.ReferencePowerW, options.ReferenceCadenceRPM, formatBalance(50+report.InterceptPP))
	fmt.Printf("Power effect:   %+.4f percentage points of left share per +100 W\n", report.PowerCoefficientPPPer100W)
	fmt.Printf("Cadence effect: %+.4f percentage points of left share per +10 rpm\n", report.CadenceCoefficientPPPer10RPM)
	if report.InteractionCoefficientPP != nil {
		fmt.Printf("Interaction:    %+.4f percentage points per (+100 W × +10 rpm)\n", *report.InteractionCoefficientPP)
	}
	fmt.Printf("\nMean observed balance: %s L/R\n", formatBalance(report.MeanLeftPercent))
	fmt.Printf("R²: %.4f\nAdjusted R²: %.4f\nRMSE: %.4f percentage points\n", report.RSquared, report.AdjustedRSquared, report.RMSEPP)
	fmt.Println("Coefficients describe association, not causation; consecutive stream samples are autocorrelated.")
}

func heatCell(left float64, color bool) string {
	content := fmt.Sprintf("%11s", fmt.Sprintf("%.1f/%.1f", left, 100-left))
	if !color {
		return content
	}
	background := 240
	deviation := left - 50
	if deviation >= 5 {
		background = 27
	} else if deviation >= 2 {
		background = 69
	} else if deviation <= -5 {
		background = 196
	} else if deviation <= -2 {
		background = 203
	}
	return fmt.Sprintf("\x1b[38;5;15;48;5;%dm%s\x1b[0m", background, content)
}
func formatRange(lower, upper float64) string { return formatNumber(lower) + "-" + formatNumber(upper) }
func formatNumber(value float64) string {
	if math.Abs(value-math.Round(value)) < 1e-9 {
		return fmt.Sprintf("%.0f", value)
	}
	return fmt.Sprintf("%.1f", value)
}
func formatBalance(left float64) string { return fmt.Sprintf("%.2f/%.2f", left, 100-left) }
func formatDuration(seconds float64) string {
	rounded := int64(math.Round(seconds))
	hours := rounded / 3600
	minutes := (rounded % 3600) / 60
	secs := rounded % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}
func activityWord(count int) string {
	if count == 1 {
		return "activity"
	}
	return "activities"
}
func isTerminal(file *os.File) bool {
	info, err := file.Stat()
	return err == nil && (info.Mode()&os.ModeCharDevice) != 0
}
