package stats

import (
	"fmt"
	"sort"

	"github.com/local/lr-analysis-go/internal/model"
)

type AnalysisOptions struct {
	BucketSizeW     float64
	MinSamples      int
	MinDurationSecs float64
	Cranks          CrankSetup
}

type AnalysisReport struct {
	Buckets           []BucketStats
	AcceptedSamples   int
	RepresentedSecs   float64
	SkippedSamples    int
	SuppressedBuckets int
}

type BucketStats struct {
	LowerW                     float64
	UpperW                     float64
	SampleCount                int
	DurationSecs               float64
	MeanPowerW                 float64
	MeanCorrectedPowerW        float64
	MeanReportedLeftPercent    float64
	MedianReportedLeftPercent  float64
	StdDevReportedLeftPercent  float64
	MeanCorrectedLeftPercent   float64
	MedianCorrectedLeftPercent float64
	StdDevCorrectedLeftPercent float64
}

type bucketAccumulator struct {
	duration       float64
	power          []float64
	correctedPower []float64
	reportedLeft   []float64
	correctedLeft  []float64
}

func (o AnalysisOptions) Validate() error {
	if !IsFinite(o.BucketSizeW) || o.BucketSizeW <= 0 {
		return fmt.Errorf("bucket size must be a positive finite number")
	}
	if !IsFinite(o.MinDurationSecs) || o.MinDurationSecs < 0 {
		return fmt.Errorf("minimum duration must be a non-negative finite number")
	}
	if o.MinSamples < 0 {
		return fmt.Errorf("minimum samples must not be negative")
	}
	return ValidateCranks(o.Cranks)
}

func Analyse(activityStreams [][]model.ActivityStream, options AnalysisOptions) (AnalysisReport, error) {
	if err := options.Validate(); err != nil {
		return AnalysisReport{}, err
	}
	accumulators := map[int]*bucketAccumulator{}
	report := AnalysisReport{}
	for _, streams := range activityStreams {
		times, err := StreamData(streams, "time")
		if err != nil {
			return AnalysisReport{}, err
		}
		watts, err := StreamData(streams, "watts")
		if err != nil {
			return AnalysisReport{}, err
		}
		balances, err := StreamData(streams, "left_right_balance")
		if err != nil {
			return AnalysisReport{}, err
		}
		fallback := MedianPositiveDelta(times)
		for index := 0; index < MaxLen(times, watts, balances); index++ {
			power, powerOK := ValueAt(watts, index)
			right, balanceOK := ValueAt(balances, index)
			if !powerOK || !balanceOK || !IsFinite(power) || power < 0 || !IsFinite(right) || right < 0 || right > 100 {
				report.SkippedSamples++
				continue
			}
			duration := SampleDuration(times, index, fallback)
			bucketIndex := int(power / options.BucketSizeW)
			accumulator := accumulators[bucketIndex]
			if accumulator == nil {
				accumulator = &bucketAccumulator{}
				accumulators[bucketIndex] = accumulator
			}
			corrected := options.Cranks.Correct(power, right)
			accumulator.duration += duration
			accumulator.power = append(accumulator.power, power)
			accumulator.correctedPower = append(accumulator.correctedPower, corrected.TotalPowerW)
			accumulator.reportedLeft = append(accumulator.reportedLeft, 100-right)
			accumulator.correctedLeft = append(accumulator.correctedLeft, corrected.LeftPercent)
			report.AcceptedSamples++
			report.RepresentedSecs += duration
		}
	}

	keys := make([]int, 0, len(accumulators))
	for key := range accumulators {
		keys = append(keys, key)
	}
	sort.Ints(keys)
	for _, key := range keys {
		accumulator := accumulators[key]
		if len(accumulator.power) < options.MinSamples || accumulator.duration < options.MinDurationSecs {
			report.SuppressedBuckets++
			continue
		}
		report.Buckets = append(report.Buckets, BucketStats{
			LowerW:      float64(key) * options.BucketSizeW,
			UpperW:      float64(key+1) * options.BucketSizeW,
			SampleCount: len(accumulator.power), DurationSecs: accumulator.duration,
			MeanPowerW: Mean(accumulator.power), MeanCorrectedPowerW: Mean(accumulator.correctedPower),
			MeanReportedLeftPercent: Mean(accumulator.reportedLeft), MedianReportedLeftPercent: Median(accumulator.reportedLeft), StdDevReportedLeftPercent: PopulationStdDev(accumulator.reportedLeft),
			MeanCorrectedLeftPercent: Mean(accumulator.correctedLeft), MedianCorrectedLeftPercent: Median(accumulator.correctedLeft), StdDevCorrectedLeftPercent: PopulationStdDev(accumulator.correctedLeft),
		})
	}
	return report, nil
}
