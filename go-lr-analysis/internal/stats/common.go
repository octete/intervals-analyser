package stats

import (
	"fmt"
	"math"
	"sort"

	"github.com/local/lr-analysis-go/internal/model"
)

type BalanceMode string

const (
	ReportedBalance  BalanceMode = "reported"
	CorrectedBalance BalanceMode = "corrected"
)

type CrankSetup struct {
	LeftMM       float64
	RightMM      float64
	ConfiguredMM float64
}

type CorrectedSample struct {
	TotalPowerW float64
	LeftPercent float64
}

func (c CrankSetup) Correct(totalPowerW, reportedRightPercent float64) CorrectedSample {
	rightFraction := reportedRightPercent / 100
	leftFraction := 1 - rightFraction
	leftPower := totalPowerW * leftFraction * c.LeftMM / c.ConfiguredMM
	rightPower := totalPowerW * rightFraction * c.RightMM / c.ConfiguredMM
	correctedTotal := leftPower + rightPower
	correctedLeft := 100 * leftFraction * c.LeftMM / (leftFraction*c.LeftMM + rightFraction*c.RightMM)
	if correctedTotal > 0 {
		correctedLeft = 100 * leftPower / correctedTotal
	}
	return CorrectedSample{TotalPowerW: correctedTotal, LeftPercent: correctedLeft}
}

func ValidateCranks(c CrankSetup) error {
	for label, value := range map[string]float64{
		"left crank length": c.LeftMM, "right crank length": c.RightMM, "configured crank length": c.ConfiguredMM,
	} {
		if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 {
			return fmt.Errorf("%s must be a positive finite number", label)
		}
	}
	return nil
}

func StreamData(streams []model.ActivityStream, kind string) ([]*float64, error) {
	for _, stream := range streams {
		if stream.Type == kind {
			return stream.NumericData()
		}
	}
	return nil, fmt.Errorf("activity does not contain a %q stream", kind)
}

func MaxLen(values ...[]*float64) int {
	maximum := 0
	for _, value := range values {
		if len(value) > maximum {
			maximum = len(value)
		}
	}
	return maximum
}

func ValueAt(values []*float64, index int) (float64, bool) {
	if index < 0 || index >= len(values) || values[index] == nil {
		return 0, false
	}
	return *values[index], true
}

func MedianPositiveDelta(times []*float64) float64 {
	deltas := make([]float64, 0, len(times))
	for i := 0; i+1 < len(times); i++ {
		if times[i] != nil && times[i+1] != nil {
			delta := *times[i+1] - *times[i]
			if delta > 0 && !math.IsInf(delta, 0) && !math.IsNaN(delta) {
				deltas = append(deltas, delta)
			}
		}
	}
	if len(deltas) == 0 {
		return 1
	}
	return Median(deltas)
}

func SampleDuration(times []*float64, index int, fallback float64) float64 {
	if index+1 < len(times) && times[index] != nil && times[index+1] != nil {
		delta := *times[index+1] - *times[index]
		if delta > 0 && !math.IsInf(delta, 0) && !math.IsNaN(delta) {
			return delta
		}
	}
	return fallback
}

func Mean(values []float64) float64 {
	var total float64
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func Median(values []float64) float64 {
	copyValues := append([]float64(nil), values...)
	sort.Float64s(copyValues)
	middle := len(copyValues) / 2
	if len(copyValues)%2 == 0 {
		return (copyValues[middle-1] + copyValues[middle]) / 2
	}
	return copyValues[middle]
}

func PopulationStdDev(values []float64) float64 {
	mean := Mean(values)
	var total float64
	for _, value := range values {
		total += math.Pow(value-mean, 2)
	}
	return math.Sqrt(total / float64(len(values)))
}

func IsFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}
