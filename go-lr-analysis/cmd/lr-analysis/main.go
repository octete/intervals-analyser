package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/local/lr-analysis-go/internal/api"
	"github.com/local/lr-analysis-go/internal/cli"
	"github.com/local/lr-analysis-go/internal/config"
	"github.com/local/lr-analysis-go/internal/model"
	"github.com/local/lr-analysis-go/internal/output"
	"github.com/local/lr-analysis-go/internal/stats"
)

func main() {
	if err := run(); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run() error {
	parsed, err := cli.Parse(os.Args[1:], os.Stderr)
	if err != nil {
		return err
	}
	settings, err := config.Load(parsed.ConfigPath)
	if err != nil {
		return err
	}
	client := api.New(settings)
	switch parsed.Command {
	case "activities":
		activities, err := latestCyclingActivities(client, parsed.Activities.Last)
		if err != nil {
			return err
		}
		output.PrintActivities(activities)
	case "analyse":
		return runAnalyse(client, *parsed.Analyse)
	case "heatmap":
		return runHeatmap(client, *parsed.Heatmap)
	case "regression":
		return runRegression(client, *parsed.Regression)
	}
	return nil
}

func runAnalyse(client *api.Client, args cli.AnalyseArgs) error {
	selected, err := selectActivities(client, args.Selection)
	if err != nil {
		return err
	}
	activities, streams, err := fetchUsableStreams(client, selected, []string{"time", "watts", "left_right_balance"})
	if err != nil {
		return err
	}
	options := stats.AnalysisOptions{BucketSizeW: args.BucketSize, MinSamples: args.MinSamples, MinDurationSecs: args.MinDuration, Cranks: cranks(args.Cranks)}
	report, err := stats.Analyse(streams, options)
	if err != nil {
		return err
	}
	output.PrintAnalysis(activities, report, options)
	return nil
}

func runHeatmap(client *api.Client, args cli.HeatmapArgs) error {
	selected, err := selectActivities(client, args.Selection)
	if err != nil {
		return err
	}
	activities, streams, err := fetchUsableStreams(client, selected, []string{"time", "watts", "cadence", "left_right_balance"})
	if err != nil {
		return err
	}
	options := stats.HeatmapOptions{PowerBucketSizeW: args.PowerBucketSize, RPMBucketSize: args.RPMBucketSize, MinSamples: args.MinSamples, MinDurationSecs: args.MinDuration, Cranks: cranks(args.Cranks), BalanceMode: args.Balance}
	report, err := stats.BuildHeatmap(streams, options)
	if err != nil {
		return err
	}
	output.PrintHeatmap(activities, report, options, args.NoColor)
	return nil
}

func runRegression(client *api.Client, args cli.RegressionArgs) error {
	selected, err := selectActivities(client, args.Selection)
	if err != nil {
		return err
	}
	activities, streams, err := fetchUsableStreams(client, selected, []string{"time", "watts", "cadence", "left_right_balance"})
	if err != nil {
		return err
	}
	options := stats.RegressionOptions{MinPowerW: args.MinPower, MinCadenceRPM: args.MinCadence, ReferencePowerW: args.ReferencePower, ReferenceCadenceRPM: args.ReferenceCadence, Interaction: args.Interaction, Cranks: cranks(args.Cranks), BalanceMode: args.Balance}
	report, err := stats.FitRegression(streams, options)
	if err != nil {
		return err
	}
	output.PrintRegression(activities, report, options)
	return nil
}

func cranks(values cli.Cranks) stats.CrankSetup {
	return stats.CrankSetup{LeftMM: values.Left, RightMM: values.Right, ConfiguredMM: values.Configured}
}

func selectActivities(client *api.Client, selection cli.Selection) ([]model.Activity, error) {
	if selection.ActivityID != "" {
		activity, err := client.Activity(selection.ActivityID)
		if err != nil {
			return nil, err
		}
		if !activity.IsCycling() {
			return nil, fmt.Errorf("activity %s is not cycling (type: %s)", activity.ID, activity.Type)
		}
		return []model.Activity{activity}, nil
	}
	if selection.From != "" {
		activities, err := client.ListActivities(selection.From, selection.To, 0)
		if err != nil {
			return nil, err
		}
		return filterCycling(activities)
	}
	return latestCyclingActivities(client, 1)
}

func latestCyclingActivities(client *api.Client, count int) ([]model.Activity, error) {
	limit := count * 10
	if limit < 100 {
		limit = 100
	}
	activities, err := client.ListActivities("1970-01-01", "", limit)
	if err != nil {
		return nil, err
	}
	cycling, err := filterCycling(activities)
	if err != nil {
		return nil, err
	}
	if len(cycling) > count {
		cycling = cycling[:count]
	}
	return cycling, nil
}
func filterCycling(activities []model.Activity) ([]model.Activity, error) {
	cycling := make([]model.Activity, 0, len(activities))
	for _, activity := range activities {
		if activity.IsCycling() {
			cycling = append(cycling, activity)
		}
	}
	if len(cycling) == 0 {
		return nil, fmt.Errorf("no cycling activities found for the requested period")
	}
	return cycling, nil
}

func fetchUsableStreams(client *api.Client, selected []model.Activity, required []string) ([]model.Activity, [][]model.ActivityStream, error) {
	activities := make([]model.Activity, 0, len(selected))
	allStreams := make([][]model.ActivityStream, 0, len(selected))
	for _, activity := range selected {
		streams, err := client.ActivityStreams(activity.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("fetch streams for %s: %w", activity.ID, err)
		}
		if !hasStreams(streams, required) {
			fmt.Fprintf(os.Stderr, "Skipping %s (%s): required streams are not all present (%v).\n", activity.ID, activity.StartDateLocal, required)
			continue
		}
		activities = append(activities, activity)
		allStreams = append(allStreams, streams)
	}
	if len(activities) == 0 {
		return nil, nil, fmt.Errorf("none of the selected cycling activities contain all required streams")
	}
	return activities, allStreams, nil
}
func hasStreams(streams []model.ActivityStream, required []string) bool {
	present := map[string]bool{}
	for _, stream := range streams {
		present[stream.Type] = true
	}
	for _, kind := range required {
		if !present[kind] {
			return false
		}
	}
	return true
}
