package analyzer

import (
	"errors"
	"fmt"
	"io"
	"math"
	"sort"
	"time"

	"github.com/muktihari/fit/decoder"
	"github.com/muktihari/fit/profile/basetype"
	"github.com/muktihari/fit/profile/filedef"
	"github.com/muktihari/fit/profile/typedef"
)

type Options struct {
	LeftCrankMM, RightCrankMM, ConfiguredCrankMM float64
	PowerBucketW, CadenceBucketRPM               float64
	MinSamples                                   int
	MinDurationSeconds                           float64
	MinRegressionPowerW, MinRegressionCadenceRPM float64
}

func DefaultOptions() Options { return Options{160, 165, 162.5, 50, 10, 5, 5, 50, 40} }

type Sample struct {
	Time                             time.Time
	PowerW, CadenceRPM, ReportedLeft float64
}
type Bucket struct {
	LowerW, UpperW, DurationSeconds, MeanPowerW float64
	Samples                                     int
	Reported, Corrected                         Distribution
}
type Distribution struct{ MeanLeft, MedianLeft, StdDev float64 }
type Cell struct {
	PowerLowerW, PowerUpperW, CadenceLowerRPM, CadenceUpperRPM, MeanLeft, MeanRight, DurationSeconds float64
	Samples                                                                                          int
}
type Regression struct {
	InterceptLeftAtReference, PowerPPPer100W, CadencePPPer10RPM, RSquared, RMSE, ReferencePowerW, ReferenceCadenceRPM float64
	Samples                                                                                                           int
}
type Report struct {
	FileName            string      `json:"fileName"`
	Samples             int         `json:"samples"`
	DurationSeconds     float64     `json:"durationSeconds"`
	Skipped             int         `json:"skipped"`
	Buckets             []Bucket    `json:"buckets"`
	ReportedHeatmap     []Cell      `json:"reportedHeatmap"`
	CorrectedHeatmap    []Cell      `json:"correctedHeatmap"`
	ReportedRegression  *Regression `json:"reportedRegression,omitempty"`
	CorrectedRegression *Regression `json:"correctedRegression,omitempty"`
}

func (o Options) Validate() error {
	for name, v := range map[string]float64{"left crank": o.LeftCrankMM, "right crank": o.RightCrankMM, "configured crank": o.ConfiguredCrankMM, "power bucket": o.PowerBucketW, "cadence bucket": o.CadenceBucketRPM} {
		if v <= 0 || math.IsNaN(v) || math.IsInf(v, 0) {
			return fmt.Errorf("%s must be positive", name)
		}
	}
	if o.MinSamples < 1 || o.MinDurationSeconds < 0 {
		return errors.New("minimum samples must be at least 1 and duration non-negative")
	}
	return nil
}

func CorrectBalance(reportedLeft, leftMM, rightMM float64) float64 {
	l := reportedLeft / 100 * leftMM
	r := (1 - reportedLeft/100) * rightMM
	return 100 * l / (l + r)
}

func PowerBucket(power, size float64) int { return int(math.Floor(power / size)) }

func Decode(r io.Reader) ([]Sample, int, error) {
	fit, err := decoder.New(r).Decode()
	if err != nil {
		return nil, 0, fmt.Errorf("decode FIT: %w", err)
	}
	activity := filedef.NewActivity(fit.Messages...)
	samples := make([]Sample, 0, len(activity.Records))
	skipped := 0
	for _, rec := range activity.Records {
		balance := uint8(rec.LeftRightBalance)
		if rec.Power == basetype.Uint16Invalid || rec.Cadence == basetype.Uint8Invalid || rec.Timestamp.IsZero() || rec.LeftRightBalance == typedef.LeftRightBalanceInvalid || balance&uint8(typedef.LeftRightBalanceRight) == 0 {
			skipped++
			continue
		}
		right := float64(balance & uint8(typedef.LeftRightBalanceMask))
		if right > 100 {
			skipped++
			continue
		}
		samples = append(samples, Sample{rec.Timestamp, float64(rec.Power), float64(rec.Cadence), 100 - right})
	}
	if len(samples) == 0 {
		return nil, skipped, errors.New("FIT file contains no records with power, cadence, and dual-sided left/right balance")
	}
	sort.Slice(samples, func(i, j int) bool { return samples[i].Time.Before(samples[j].Time) })
	return samples, skipped, nil
}

func Analyse(fileName string, samples []Sample, skipped int, o Options) (Report, error) {
	if err := o.Validate(); err != nil {
		return Report{}, err
	}
	durations := make([]float64, len(samples))
	var ds []float64
	for i := 0; i+1 < len(samples); i++ {
		d := samples[i+1].Time.Sub(samples[i].Time).Seconds()
		if d > 0 && d <= 30 {
			ds = append(ds, d)
		}
	}
	fallback := 1.0
	if len(ds) > 0 {
		fallback = median(ds)
	}
	for i := range samples {
		durations[i] = fallback
		if i+1 < len(samples) {
			d := samples[i+1].Time.Sub(samples[i].Time).Seconds()
			if d > 0 && d <= 30 {
				durations[i] = d
			}
		}
	}
	type acc struct {
		power, reported, corrected []float64
		duration                   float64
	}
	bs := map[int]*acc{}
	type key struct{ p, c int }
	hs := map[key]*acc{}
	report := Report{
		FileName: fileName, Samples: len(samples), Skipped: skipped,
		Buckets: []Bucket{}, ReportedHeatmap: []Cell{}, CorrectedHeatmap: []Cell{},
	}
	for i, s := range samples {
		report.DurationSeconds += durations[i]
		bi := PowerBucket(s.PowerW, o.PowerBucketW)
		if bs[bi] == nil {
			bs[bi] = &acc{}
		}
		b := bs[bi]
		b.power = append(b.power, s.PowerW)
		b.reported = append(b.reported, s.ReportedLeft)
		corr := CorrectBalance(s.ReportedLeft, o.LeftCrankMM, o.RightCrankMM)
		b.corrected = append(b.corrected, corr)
		b.duration += durations[i]
		k := key{bi, int(s.CadenceRPM / o.CadenceBucketRPM)}
		if hs[k] == nil {
			hs[k] = &acc{}
		}
		h := hs[k]
		h.reported = append(h.reported, s.ReportedLeft)
		h.corrected = append(h.corrected, corr)
		h.duration += durations[i]
	}
	var keys []int
	for k := range bs {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	for _, k := range keys {
		a := bs[k]
		if len(a.power) < o.MinSamples || a.duration < o.MinDurationSeconds {
			continue
		}
		report.Buckets = append(report.Buckets, Bucket{float64(k) * o.PowerBucketW, float64(k+1) * o.PowerBucketW, a.duration, mean(a.power), len(a.power), dist(a.reported), dist(a.corrected)})
	}
	var hkeys []key
	for k := range hs {
		hkeys = append(hkeys, k)
	}
	sort.Slice(hkeys, func(i, j int) bool {
		if hkeys[i].c == hkeys[j].c {
			return hkeys[i].p < hkeys[j].p
		}
		return hkeys[i].c < hkeys[j].c
	})
	for _, k := range hkeys {
		a := hs[k]
		if len(a.reported) < o.MinSamples || a.duration < o.MinDurationSeconds {
			continue
		}
		mk := func(v []float64) Cell {
			l := mean(v)
			return Cell{float64(k.p) * o.PowerBucketW, float64(k.p+1) * o.PowerBucketW, float64(k.c) * o.CadenceBucketRPM, float64(k.c+1) * o.CadenceBucketRPM, l, 100 - l, a.duration, len(v)}
		}
		report.ReportedHeatmap = append(report.ReportedHeatmap, mk(a.reported))
		report.CorrectedHeatmap = append(report.CorrectedHeatmap, mk(a.corrected))
	}
	report.ReportedRegression = regression(samples, false, o)
	report.CorrectedRegression = regression(samples, true, o)
	return report, nil
}

func dist(v []float64) Distribution { return Distribution{mean(v), median(v), stddev(v)} }
func mean(v []float64) float64 {
	var s float64
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}
func median(v []float64) float64 {
	x := append([]float64(nil), v...)
	sort.Float64s(x)
	m := len(x) / 2
	if len(x)%2 == 0 {
		return (x[m-1] + x[m]) / 2
	}
	return x[m]
}
func stddev(v []float64) float64 {
	m := mean(v)
	var s float64
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(len(v)))
}

func regression(samples []Sample, corrected bool, o Options) *Regression {
	const rp, rc = 200.0, 85.0
	var rows [][3]float64
	var ys []float64
	for _, s := range samples {
		if s.PowerW < o.MinRegressionPowerW || s.CadenceRPM < o.MinRegressionCadenceRPM {
			continue
		}
		l := s.ReportedLeft
		if corrected {
			l = CorrectBalance(l, o.LeftCrankMM, o.RightCrankMM)
		}
		rows = append(rows, [3]float64{1, (s.PowerW - rp) / 100, (s.CadenceRPM - rc) / 10})
		ys = append(ys, l)
	}
	if len(rows) < 4 {
		return nil
	}
	var a [3][3]float64
	var b [3]float64
	for i, x := range rows {
		for r := 0; r < 3; r++ {
			b[r] += x[r] * ys[i]
			for c := 0; c < 3; c++ {
				a[r][c] += x[r] * x[c]
			}
		}
	}
	coef, ok := solve3(a, b)
	if !ok {
		return nil
	}
	m := mean(ys)
	var se, st float64
	for i, x := range rows {
		p := coef[0] + coef[1]*x[1] + coef[2]*x[2]
		se += (ys[i] - p) * (ys[i] - p)
		st += (ys[i] - m) * (ys[i] - m)
	}
	r2 := 0.0
	if st > 0 {
		r2 = 1 - se/st
	}
	return &Regression{coef[0], coef[1], coef[2], r2, math.Sqrt(se / float64(len(rows))), rp, rc, len(rows)}
}
func solve3(a [3][3]float64, b [3]float64) ([3]float64, bool) {
	for p := 0; p < 3; p++ {
		best := p
		for r := p + 1; r < 3; r++ {
			if math.Abs(a[r][p]) > math.Abs(a[best][p]) {
				best = r
			}
		}
		a[p], a[best] = a[best], a[p]
		b[p], b[best] = b[best], b[p]
		if math.Abs(a[p][p]) < 1e-10 {
			return [3]float64{}, false
		}
		d := a[p][p]
		for c := p; c < 3; c++ {
			a[p][c] /= d
		}
		b[p] /= d
		for r := 0; r < 3; r++ {
			if r == p {
				continue
			}
			f := a[r][p]
			for c := p; c < 3; c++ {
				a[r][c] -= f * a[p][c]
			}
			b[r] -= f * b[p]
		}
	}
	return b, true
}
