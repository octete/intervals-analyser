package analyzer

import (
	"math"
	"testing"
)

func TestPowerBucket(t *testing.T) {
	for _, tc := range []struct {
		p, s float64
		want int
	}{{0, 50, 0}, {49.99, 50, 0}, {50, 50, 1}, {149, 25, 5}} {
		if got := PowerBucket(tc.p, tc.s); got != tc.want {
			t.Fatalf("PowerBucket(%v,%v)=%d want %d", tc.p, tc.s, got, tc.want)
		}
	}
}
func TestCrankCorrection(t *testing.T) {
	got := CorrectBalance(50, 160, 165)
	want := 100 * 80.0 / 162.5
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("got %.9f want %.9f", got, want)
	}
	if got >= 50 {
		t.Fatalf("shorter left crank should lower corrected left share, got %f", got)
	}
}
