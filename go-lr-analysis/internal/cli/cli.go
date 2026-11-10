package cli

import (
	"flag"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/local/lr-analysis-go/internal/stats"
)

type Selection struct {
	ActivityID string
	Last       bool
	From       string
	To         string
}

type Cranks struct{ Left, Right, Configured float64 }
type ActivitiesArgs struct{ Last int }
type AnalyseArgs struct {
	Selection   Selection
	BucketSize  float64
	MinSamples  int
	MinDuration float64
	Cranks      Cranks
}
type HeatmapArgs struct {
	Selection                      Selection
	PowerBucketSize, RPMBucketSize float64
	Balance                        stats.BalanceMode
	MinSamples                     int
	MinDuration                    float64
	Cranks                         Cranks
	NoColor                        bool
}
type RegressionArgs struct {
	Selection                                              Selection
	Balance                                                stats.BalanceMode
	MinPower, MinCadence, ReferencePower, ReferenceCadence float64
	Interaction                                            bool
	Cranks                                                 Cranks
}
type Parsed struct {
	Command, ConfigPath string
	Activities          *ActivitiesArgs
	Analyse             *AnalyseArgs
	Heatmap             *HeatmapArgs
	Regression          *RegressionArgs
}

func Parse(args []string, stderr io.Writer) (Parsed, error) {
	configPath, args, err := extractGlobalConfig(args)
	if err != nil {
		return Parsed{}, err
	}
	if len(args) == 0 {
		return Parsed{}, fmt.Errorf("expected command: activities, analyse, heatmap, or regression")
	}
	parsed := Parsed{Command: args[0], ConfigPath: configPath}
	switch args[0] {
	case "activities":
		values := &ActivitiesArgs{}
		set := newSet("activities", stderr)
		set.IntVar(&values.Last, "last", 20, "number of recent cycling activities")
		if err := set.Parse(args[1:]); err != nil {
			return Parsed{}, err
		}
		if values.Last < 1 {
			return Parsed{}, fmt.Errorf("--last must be at least 1")
		}
		parsed.Activities = values
	case "analyse":
		values := &AnalyseArgs{BucketSize: 50, MinSamples: 1, Cranks: defaultCranks()}
		set := newSet("analyse", stderr)
		bindSelection(set, &values.Selection)
		set.Float64Var(&values.BucketSize, "bucket-size", 50, "power bucket width in watts")
		set.IntVar(&values.MinSamples, "min-samples", 1, "minimum samples per bucket")
		set.Float64Var(&values.MinDuration, "min-duration", 0, "minimum seconds per bucket")
		bindCranks(set, &values.Cranks)
		if err := set.Parse(args[1:]); err != nil {
			return Parsed{}, err
		}
		if err := validateSelection(values.Selection); err != nil {
			return Parsed{}, err
		}
		parsed.Analyse = values
	case "heatmap":
		values := &HeatmapArgs{PowerBucketSize: 50, RPMBucketSize: 10, Balance: stats.CorrectedBalance, MinSamples: 1, Cranks: defaultCranks()}
		set := newSet("heatmap", stderr)
		bindSelection(set, &values.Selection)
		set.Float64Var(&values.PowerBucketSize, "power-bucket-size", 50, "power bucket width in watts")
		set.Float64Var(&values.RPMBucketSize, "rpm-bucket-size", 10, "cadence bucket width in rpm")
		balance := string(values.Balance)
		set.StringVar(&balance, "balance", balance, "reported or corrected")
		set.IntVar(&values.MinSamples, "min-samples", 1, "minimum samples per cell")
		set.Float64Var(&values.MinDuration, "min-duration", 0, "minimum seconds per cell")
		bindCranks(set, &values.Cranks)
		set.BoolVar(&values.NoColor, "no-color", false, "disable ANSI colours")
		if err := set.Parse(args[1:]); err != nil {
			return Parsed{}, err
		}
		values.Balance = stats.BalanceMode(balance)
		if err := validateSelection(values.Selection); err != nil {
			return Parsed{}, err
		}
		parsed.Heatmap = values
	case "regression":
		values := &RegressionArgs{Balance: stats.CorrectedBalance, MinPower: 50, MinCadence: 1, ReferencePower: 200, ReferenceCadence: 85, Cranks: defaultCranks()}
		set := newSet("regression", stderr)
		bindSelection(set, &values.Selection)
		balance := string(values.Balance)
		set.StringVar(&balance, "balance", balance, "reported or corrected")
		set.Float64Var(&values.MinPower, "min-power", 50, "minimum power in watts")
		set.Float64Var(&values.MinCadence, "min-cadence", 1, "minimum cadence in rpm")
		set.Float64Var(&values.ReferencePower, "reference-power", 200, "reference power in watts")
		set.Float64Var(&values.ReferenceCadence, "reference-cadence", 85, "reference cadence in rpm")
		set.BoolVar(&values.Interaction, "interaction", false, "include power-cadence interaction")
		bindCranks(set, &values.Cranks)
		if err := set.Parse(args[1:]); err != nil {
			return Parsed{}, err
		}
		values.Balance = stats.BalanceMode(balance)
		if err := validateSelection(values.Selection); err != nil {
			return Parsed{}, err
		}
		parsed.Regression = values
	default:
		return Parsed{}, fmt.Errorf("unknown command %q; expected activities, analyse, heatmap, or regression", args[0])
	}
	return parsed, nil
}

func newSet(name string, stderr io.Writer) *flag.FlagSet {
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	set.SetOutput(stderr)
	return set
}
func defaultCranks() Cranks { return Cranks{Left: 160, Right: 165, Configured: 162.5} }
func bindCranks(set *flag.FlagSet, values *Cranks) {
	set.Float64Var(&values.Left, "left-crank", values.Left, "actual left crank length in mm")
	set.Float64Var(&values.Right, "right-crank", values.Right, "actual right crank length in mm")
	set.Float64Var(&values.Configured, "configured-crank", values.Configured, "power-meter crank setting in mm")
}
func bindSelection(set *flag.FlagSet, selection *Selection) {
	set.StringVar(&selection.ActivityID, "activity-id", "", "Intervals.icu activity ID")
	set.BoolVar(&selection.Last, "last", false, "most recent cycling activity")
	set.StringVar(&selection.From, "from", "", "first date YYYY-MM-DD")
	set.StringVar(&selection.To, "to", "", "last date YYYY-MM-DD")
}

func validateSelection(selection Selection) error {
	if selection.ActivityID != "" && (selection.Last || selection.From != "" || selection.To != "") {
		return fmt.Errorf("--activity-id cannot be combined with --last, --from, or --to")
	}
	if selection.Last && (selection.From != "" || selection.To != "") {
		return fmt.Errorf("--last cannot be combined with --from or --to")
	}
	if selection.To != "" && selection.From == "" {
		return fmt.Errorf("--to requires --from")
	}
	var from, timeTo time.Time
	var err error
	if selection.From != "" {
		from, err = time.Parse("2006-01-02", selection.From)
		if err != nil {
			return fmt.Errorf("invalid --from date: %w", err)
		}
	}
	if selection.To != "" {
		timeTo, err = time.Parse("2006-01-02", selection.To)
		if err != nil {
			return fmt.Errorf("invalid --to date: %w", err)
		}
		if from.After(timeTo) {
			return fmt.Errorf("--from must not be later than --to")
		}
	}
	return nil
}

func extractGlobalConfig(args []string) (string, []string, error) {
	remaining := make([]string, 0, len(args))
	var config string
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if arg == "--config" {
			if index+1 >= len(args) {
				return "", nil, fmt.Errorf("--config requires a path")
			}
			config = args[index+1]
			index++
			continue
		}
		if strings.HasPrefix(arg, "--config=") {
			config = strings.TrimPrefix(arg, "--config=")
			continue
		}
		remaining = append(remaining, arg)
	}
	return config, remaining, nil
}
