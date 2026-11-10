package stats

import (
	"encoding/json"
	"math"
	"testing"

	"github.com/local/lr-analysis-go/internal/model"
)

func stream(kind string, values any) model.ActivityStream {
	data, _ := json.Marshal(values)
	return model.ActivityStream{Type: kind, Data: data}
}

func testCranks() CrankSetup { return CrankSetup{LeftMM: 160, RightMM: 165, ConfiguredMM: 162.5} }

func closeEnough(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

func TestCrankCorrection(t *testing.T) {
	corrected := testCranks().Correct(200, 50)
	if !closeEnough(corrected.TotalPowerW, 200) {
		t.Fatalf("total power = %v", corrected.TotalPowerW)
	}
	expected := 160.0 / 325.0 * 100
	if !closeEnough(corrected.LeftPercent, expected) {
		t.Fatalf("left = %v, want %v", corrected.LeftPercent, expected)
	}
}

func TestPowerBucketsAreLowerInclusive(t *testing.T) {
	streams := []model.ActivityStream{stream("time", []float64{0, 1, 2, 3}), stream("watts", []float64{0, 49.999, 50, 100}), stream("left_right_balance", []float64{50, 50, 50, 50})}
	report, err := Analyse([][]model.ActivityStream{streams}, AnalysisOptions{BucketSizeW: 50, MinSamples: 1, Cranks: testCranks()})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Buckets) != 3 || report.Buckets[0].SampleCount != 2 || report.Buckets[1].LowerW != 50 || report.Buckets[2].LowerW != 100 {
		t.Fatalf("unexpected buckets: %+v", report.Buckets)
	}
}

func TestHeatmapPowerAndCadenceBuckets(t *testing.T) {
	streams := []model.ActivityStream{stream("time", []float64{0, 1, 2}), stream("watts", []float64{49.9, 50, 100}), stream("cadence", []float64{79.9, 80, 90}), stream("left_right_balance", []float64{50, 51, 52})}
	report, err := BuildHeatmap([][]model.ActivityStream{streams}, HeatmapOptions{PowerBucketSizeW: 50, RPMBucketSize: 10, MinSamples: 1, Cranks: testCranks(), BalanceMode: ReportedBalance})
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []CellKey{{0, 7}, {1, 8}, {2, 9}} {
		if _, ok := report.Cells[key]; !ok {
			t.Errorf("missing cell %+v", key)
		}
	}
}

func TestRegressionRecoversKnownCoefficients(t *testing.T) {
	powers := []float64{100, 100, 200, 200, 300, 300}
	cadences := []float64{70, 90, 70, 90, 70, 90}
	right := make([]float64, len(powers))
	for i := range powers {
		deviation := 1.2 + 0.8*((powers[i]-200)/100) - 0.3*((cadences[i]-85)/10)
		right[i] = 50 - deviation
	}
	streams := []model.ActivityStream{stream("time", []float64{0, 1, 2, 3, 4, 5}), stream("watts", powers), stream("cadence", cadences), stream("left_right_balance", right)}
	report, err := FitRegression([][]model.ActivityStream{streams}, RegressionOptions{ReferencePowerW: 200, ReferenceCadenceRPM: 85, Cranks: testCranks(), BalanceMode: ReportedBalance})
	if err != nil {
		t.Fatal(err)
	}
	if !closeEnough(report.InterceptPP, 1.2) || !closeEnough(report.PowerCoefficientPPPer100W, 0.8) || !closeEnough(report.CadenceCoefficientPPPer10RPM, -0.3) || !closeEnough(report.RSquared, 1) {
		t.Fatalf("unexpected regression: %+v", report)
	}
}

func TestRegressionRecoversInteraction(t *testing.T) {
	var powers, cadences, right []float64
	for _, power := range []float64{100, 200, 300} {
		for _, cadence := range []float64{70, 85, 100} {
			p := (power - 200) / 100
			c := (cadence - 85) / 10
			deviation := 0.5 + 0.4*p - 0.2*c + 0.7*p*c
			powers = append(powers, power)
			cadences = append(cadences, cadence)
			right = append(right, 50-deviation)
		}
	}
	streams := []model.ActivityStream{stream("time", []float64{0, 1, 2, 3, 4, 5, 6, 7, 8}), stream("watts", powers), stream("cadence", cadences), stream("left_right_balance", right)}
	report, err := FitRegression([][]model.ActivityStream{streams}, RegressionOptions{ReferencePowerW: 200, ReferenceCadenceRPM: 85, Interaction: true, Cranks: testCranks(), BalanceMode: ReportedBalance})
	if err != nil {
		t.Fatal(err)
	}
	if report.InteractionCoefficientPP == nil || !closeEnough(*report.InteractionCoefficientPP, 0.7) {
		t.Fatalf("unexpected interaction: %+v", report.InteractionCoefficientPP)
	}
}
