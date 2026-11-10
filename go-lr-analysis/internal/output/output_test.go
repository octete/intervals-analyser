package output

import (
	"strings"
	"testing"
)

func TestHeatCellShowsBothSides(t *testing.T) {
	if got := strings.TrimSpace(heatCell(58, false)); got != "58.0/42.0" {
		t.Fatalf("got %q", got)
	}
}
