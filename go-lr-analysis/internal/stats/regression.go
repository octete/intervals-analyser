package stats

import (
	"fmt"
	"math"

	"github.com/local/lr-analysis-go/internal/model"
)

type RegressionOptions struct {
	MinPowerW, MinCadenceRPM, ReferencePowerW, ReferenceCadenceRPM float64
	Interaction                                                    bool
	Cranks                                                         CrankSetup
	BalanceMode                                                    BalanceMode
}
type RegressionReport struct {
	InterceptPP, PowerCoefficientPPPer100W, CadenceCoefficientPPPer10RPM float64
	InteractionCoefficientPP                                             *float64
	RSquared, AdjustedRSquared, RMSEPP, MeanLeftPercent                  float64
	SampleCount                                                          int
	RepresentedSecs                                                      float64
	InvalidSamples, FilteredSamples                                      int
	MinPowerW, MaxPowerW, MinCadenceRPM, MaxCadenceRPM                   float64
}
type observation struct{ power, cadence, left, duration float64 }

func (o RegressionOptions) Validate() error {
	for label, value := range map[string]float64{"minimum power": o.MinPowerW, "minimum cadence": o.MinCadenceRPM, "reference power": o.ReferencePowerW, "reference cadence": o.ReferenceCadenceRPM} {
		if !IsFinite(value) || value < 0 {
			return fmt.Errorf("%s must be a non-negative finite number", label)
		}
	}
	if o.BalanceMode != ReportedBalance && o.BalanceMode != CorrectedBalance {
		return fmt.Errorf("balance must be reported or corrected")
	}
	return ValidateCranks(o.Cranks)
}

func FitRegression(activityStreams [][]model.ActivityStream, options RegressionOptions) (RegressionReport, error) {
	if err := options.Validate(); err != nil {
		return RegressionReport{}, err
	}
	var observations []observation
	report := RegressionReport{MinPowerW: math.Inf(1), MaxPowerW: math.Inf(-1), MinCadenceRPM: math.Inf(1), MaxCadenceRPM: math.Inf(-1)}
	for _, streams := range activityStreams {
		times, err := StreamData(streams, "time")
		if err != nil {
			return RegressionReport{}, err
		}
		watts, err := StreamData(streams, "watts")
		if err != nil {
			return RegressionReport{}, err
		}
		cadence, err := StreamData(streams, "cadence")
		if err != nil {
			return RegressionReport{}, err
		}
		balances, err := StreamData(streams, "left_right_balance")
		if err != nil {
			return RegressionReport{}, err
		}
		fallback := MedianPositiveDelta(times)
		for index := 0; index < MaxLen(times, watts, cadence, balances); index++ {
			power, powerOK := ValueAt(watts, index)
			rpm, rpmOK := ValueAt(cadence, index)
			right, balanceOK := ValueAt(balances, index)
			if !powerOK || !rpmOK || !balanceOK || !IsFinite(power) || power < 0 || !IsFinite(rpm) || rpm < 0 || !IsFinite(right) || right < 0 || right > 100 {
				report.InvalidSamples++
				continue
			}
			if power < options.MinPowerW || rpm < options.MinCadenceRPM {
				report.FilteredSamples++
				continue
			}
			left := 100 - right
			if options.BalanceMode == CorrectedBalance {
				left = options.Cranks.Correct(power, right).LeftPercent
			}
			observations = append(observations, observation{power: power, cadence: rpm, left: left, duration: SampleDuration(times, index, fallback)})
		}
	}
	parameterCount := 3
	if options.Interaction {
		parameterCount = 4
	}
	if len(observations) <= parameterCount {
		return RegressionReport{}, fmt.Errorf("not enough valid samples for regression: need more than %d, got %d", parameterCount, len(observations))
	}
	matrix := make([][]float64, parameterCount)
	for i := range matrix {
		matrix[i] = make([]float64, parameterCount)
	}
	values := make([]float64, parameterCount)
	for _, observation := range observations {
		x := regressionPredictors(observation, options)
		y := observation.left - 50
		for row := 0; row < parameterCount; row++ {
			values[row] += x[row] * y
			for column := 0; column < parameterCount; column++ {
				matrix[row][column] += x[row] * x[column]
			}
		}
	}
	coefficients, err := solve(matrix, values)
	if err != nil {
		return RegressionReport{}, fmt.Errorf("regression could not be fitted; power and cadence may not vary independently enough: %w", err)
	}
	var meanLeft float64
	for _, observation := range observations {
		meanLeft += observation.left
	}
	meanLeft /= float64(len(observations))
	var squaredError, totalSquares, duration float64
	for _, observation := range observations {
		predicted := dot(coefficients, regressionPredictors(observation, options))
		residual := observation.left - 50 - predicted
		squaredError += residual * residual
		totalSquares += math.Pow(observation.left-meanLeft, 2)
		duration += observation.duration
		report.MinPowerW = math.Min(report.MinPowerW, observation.power)
		report.MaxPowerW = math.Max(report.MaxPowerW, observation.power)
		report.MinCadenceRPM = math.Min(report.MinCadenceRPM, observation.cadence)
		report.MaxCadenceRPM = math.Max(report.MaxCadenceRPM, observation.cadence)
	}
	r2 := 0.0
	if totalSquares <= math.SmallestNonzeroFloat64 {
		if squaredError <= math.SmallestNonzeroFloat64 {
			r2 = 1
		}
	} else {
		r2 = 1 - squaredError/totalSquares
	}
	report.InterceptPP = coefficients[0]
	report.PowerCoefficientPPPer100W = coefficients[1]
	report.CadenceCoefficientPPPer10RPM = coefficients[2]
	if options.Interaction {
		value := coefficients[3]
		report.InteractionCoefficientPP = &value
	}
	report.RSquared = r2
	report.AdjustedRSquared = 1 - (1-r2)*float64(len(observations)-1)/float64(len(observations)-parameterCount)
	report.RMSEPP = math.Sqrt(squaredError / float64(len(observations)))
	report.MeanLeftPercent = meanLeft
	report.SampleCount = len(observations)
	report.RepresentedSecs = duration
	return report, nil
}

func regressionPredictors(observation observation, options RegressionOptions) []float64 {
	p := (observation.power - options.ReferencePowerW) / 100
	c := (observation.cadence - options.ReferenceCadenceRPM) / 10
	values := []float64{1, p, c}
	if options.Interaction {
		values = append(values, p*c)
	}
	return values
}
func dot(a, b []float64) float64 {
	var total float64
	for i := range a {
		total += a[i] * b[i]
	}
	return total
}
func solve(matrix [][]float64, values []float64) ([]float64, error) {
	size := len(values)
	for pivot := 0; pivot < size; pivot++ {
		best := pivot
		for row := pivot + 1; row < size; row++ {
			if math.Abs(matrix[row][pivot]) > math.Abs(matrix[best][pivot]) {
				best = row
			}
		}
		matrix[pivot], matrix[best] = matrix[best], matrix[pivot]
		values[pivot], values[best] = values[best], values[pivot]
		pv := matrix[pivot][pivot]
		if !IsFinite(pv) || math.Abs(pv) < 1e-10 {
			return nil, fmt.Errorf("singular regression matrix")
		}
		for column := pivot; column < size; column++ {
			matrix[pivot][column] /= pv
		}
		values[pivot] /= pv
		pivotRow := append([]float64(nil), matrix[pivot]...)
		for row := 0; row < size; row++ {
			if row == pivot {
				continue
			}
			factor := matrix[row][pivot]
			for column := pivot; column < size; column++ {
				matrix[row][column] -= factor * pivotRow[column]
			}
			values[row] -= factor * values[pivot]
		}
	}
	return values, nil
}
