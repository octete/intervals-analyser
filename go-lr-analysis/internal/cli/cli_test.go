package cli

import (
	"io"
	"testing"
)

func TestParseActivityID(t *testing.T) {
	parsed, err := Parse([]string{"analyse", "--activity-id", "i12345678"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Analyse.Selection.ActivityID != "i12345678" {
		t.Fatalf("activity id = %q", parsed.Analyse.Selection.ActivityID)
	}
}
func TestActivityIDConflictsWithDate(t *testing.T) {
	_, err := Parse([]string{"analyse", "--activity-id", "i12345678", "--from", "2026-01-01"}, io.Discard)
	if err == nil {
		t.Fatal("expected conflict")
	}
}
func TestParseHeatmapBuckets(t *testing.T) {
	parsed, err := Parse([]string{"heatmap", "--power-bucket-size", "25", "--rpm-bucket-size", "5"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Heatmap.PowerBucketSize != 25 || parsed.Heatmap.RPMBucketSize != 5 {
		t.Fatalf("unexpected args: %+v", parsed.Heatmap)
	}
}
func TestParseRegressionInteraction(t *testing.T) {
	parsed, err := Parse([]string{"regression", "--min-power", "75", "--interaction"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Regression.MinPower != 75 || !parsed.Regression.Interaction {
		t.Fatalf("unexpected args: %+v", parsed.Regression)
	}
}
