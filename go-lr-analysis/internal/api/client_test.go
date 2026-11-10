package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/local/lr-analysis-go/internal/config"
)

func TestListActivitiesUsesBasicAuthAndDocumentedEndpoint(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		username, password, ok := request.BasicAuth()
		if !ok || username != "API_KEY" || password != "secret" {
			t.Errorf("unexpected auth: %q %q %v", username, password, ok)
		}
		if request.URL.Path != "/api/v1/athlete/123/activities" || request.URL.Query().Get("oldest") != "2026-01-01" {
			t.Errorf("unexpected request URL: %s", request.URL.String())
		}
		fmt.Fprint(response, `[{"id":"i1","type":"Ride","start_date_local":"2026-01-01T10:00:00"}]`)
	}))
	defer server.Close()

	client := New(config.Config{APIKey: "secret", AthleteID: "123", BaseURL: server.URL})
	activities, err := client.ListActivities("2026-01-01", "", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(activities) != 1 || activities[0].ID != "i1" {
		t.Fatalf("unexpected activities: %+v", activities)
	}
}

func TestActivityStreamsRequestsExactStreamNames(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/v1/activity/i42/streams.json" {
			t.Errorf("unexpected path: %s", request.URL.Path)
		}
		if got := request.URL.Query().Get("types"); got != "time,watts,cadence,left_right_balance" {
			t.Errorf("unexpected stream types: %q", got)
		}
		fmt.Fprint(response, `[{"type":"watts","data":[100,110]}]`)
	}))
	defer server.Close()

	client := New(config.Config{APIKey: "secret", AthleteID: "123", BaseURL: server.URL})
	streams, err := client.ActivityStreams("i42")
	if err != nil {
		t.Fatal(err)
	}
	if len(streams) != 1 || streams[0].Type != "watts" {
		t.Fatalf("unexpected streams: %+v", streams)
	}
}
