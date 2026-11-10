package model

import (
	"encoding/json"
	"fmt"
	"strings"
)

type Activity struct {
	ID               string   `json:"id"`
	Name             string   `json:"name"`
	StartDateLocal   string   `json:"start_date_local"`
	Type             string   `json:"type"`
	MovingTime       *uint64  `json:"moving_time"`
	AverageWatts     *float64 `json:"icu_average_watts"`
	AverageLRBalance *float64 `json:"avg_lr_balance"`
}

func (a Activity) IsCycling() bool {
	t := strings.ToLower(a.Type)
	return strings.Contains(t, "ride") || strings.Contains(t, "cycling") ||
		t == "handcycle" || t == "velomobile"
}

type ActivityStream struct {
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

func (s ActivityStream) NumericData() ([]*float64, error) {
	var values []*float64
	if err := json.Unmarshal(s.Data, &values); err != nil {
		return nil, fmt.Errorf("stream %q data is not a numeric array: %w", s.Type, err)
	}
	return values, nil
}
