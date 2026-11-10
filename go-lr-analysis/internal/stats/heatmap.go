package stats

import (
	"fmt"

	"github.com/local/lr-analysis-go/internal/model"
)

type HeatmapOptions struct {
	PowerBucketSizeW float64
	RPMBucketSize    float64
	MinSamples       int
	MinDurationSecs  float64
	Cranks           CrankSetup
	BalanceMode      BalanceMode
}

type CellKey struct{ Power, RPM int }
type HeatmapCell struct{ MeanLeftPercent float64 }
type HeatmapReport struct {
	Cells                                            map[CellKey]HeatmapCell
	AcceptedSamples, SkippedSamples, SuppressedCells int
	RepresentedSecs                                  float64
}
type cellAccumulator struct {
	count             int
	duration, leftSum float64
}

func (o HeatmapOptions) Validate() error {
	if !IsFinite(o.PowerBucketSizeW) || o.PowerBucketSizeW <= 0 {
		return fmt.Errorf("power bucket size must be a positive finite number")
	}
	if !IsFinite(o.RPMBucketSize) || o.RPMBucketSize <= 0 {
		return fmt.Errorf("rpm bucket size must be a positive finite number")
	}
	if !IsFinite(o.MinDurationSecs) || o.MinDurationSecs < 0 {
		return fmt.Errorf("minimum duration must be a non-negative finite number")
	}
	if o.BalanceMode != ReportedBalance && o.BalanceMode != CorrectedBalance {
		return fmt.Errorf("balance must be reported or corrected")
	}
	return ValidateCranks(o.Cranks)
}

func BuildHeatmap(activityStreams [][]model.ActivityStream, options HeatmapOptions) (HeatmapReport, error) {
	if err := options.Validate(); err != nil {
		return HeatmapReport{}, err
	}
	accumulators := map[CellKey]*cellAccumulator{}
	report := HeatmapReport{Cells: map[CellKey]HeatmapCell{}}
	for _, streams := range activityStreams {
		times, err := StreamData(streams, "time")
		if err != nil {
			return HeatmapReport{}, err
		}
		watts, err := StreamData(streams, "watts")
		if err != nil {
			return HeatmapReport{}, err
		}
		cadence, err := StreamData(streams, "cadence")
		if err != nil {
			return HeatmapReport{}, err
		}
		balances, err := StreamData(streams, "left_right_balance")
		if err != nil {
			return HeatmapReport{}, err
		}
		fallback := MedianPositiveDelta(times)
		for index := 0; index < MaxLen(times, watts, cadence, balances); index++ {
			power, powerOK := ValueAt(watts, index)
			rpm, rpmOK := ValueAt(cadence, index)
			right, balanceOK := ValueAt(balances, index)
			if !powerOK || !rpmOK || !balanceOK || !IsFinite(power) || power < 0 || !IsFinite(rpm) || rpm < 0 || !IsFinite(right) || right < 0 || right > 100 {
				report.SkippedSamples++
				continue
			}
			left := 100 - right
			if options.BalanceMode == CorrectedBalance {
				left = options.Cranks.Correct(power, right).LeftPercent
			}
			key := CellKey{Power: int(power / options.PowerBucketSizeW), RPM: int(rpm / options.RPMBucketSize)}
			accumulator := accumulators[key]
			if accumulator == nil {
				accumulator = &cellAccumulator{}
				accumulators[key] = accumulator
			}
			duration := SampleDuration(times, index, fallback)
			accumulator.count++
			accumulator.duration += duration
			accumulator.leftSum += left
			report.AcceptedSamples++
			report.RepresentedSecs += duration
		}
	}
	for key, accumulator := range accumulators {
		if accumulator.count < options.MinSamples || accumulator.duration < options.MinDurationSecs {
			report.SuppressedCells++
			continue
		}
		report.Cells[key] = HeatmapCell{MeanLeftPercent: accumulator.leftSum / float64(accumulator.count)}
	}
	return report, nil
}
