package espn

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// mockTransport is a fake http.RoundTripper that returns canned responses.
type mockTransport struct {
	statusCode int
	body       string
}

func (m *mockTransport) RoundTrip(_ *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: m.statusCode,
		Body:       io.NopCloser(strings.NewReader(m.body)),
		Header:     make(http.Header),
	}, nil
}

func TestFetchWeek_Success(t *testing.T) {
	sb := ScoreboardResponse{
		Events: []Event{
			{ID: "123", Date: "2025-09-07T17:00:00Z"},
		},
	}
	body, _ := json.Marshal(sb)

	c := &Client{
		http: &http.Client{Transport: &mockTransport{statusCode: 200, body: string(body)}},
	}

	got, err := c.FetchWeek(1, 2025, 2)
	if err != nil {
		t.Fatalf("FetchWeek: %v", err)
	}
	if len(got.Events) != 1 {
		t.Errorf("expected 1 event, got %d", len(got.Events))
	}
}

func TestFetchWeek_NonOKStatus(t *testing.T) {
	c := &Client{
		http: &http.Client{Transport: &mockTransport{statusCode: 503, body: ""}},
	}
	_, err := c.FetchWeek(1, 2025, 2)
	if err == nil {
		t.Fatal("expected error for non-200 status")
	}
}

func TestFetchWeek_InvalidJSON(t *testing.T) {
	c := &Client{
		http: &http.Client{Transport: &mockTransport{statusCode: 200, body: "not json"}},
	}
	_, err := c.FetchWeek(1, 2025, 2)
	if err == nil {
		t.Fatal("expected error for invalid JSON body")
	}
}
