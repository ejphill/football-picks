package espn

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const baseURL = "https://site.api.espn.com/apis/site/v2/sports/football/nfl/scoreboard"

// Client hits the ESPN scoreboard API.
type Client struct {
	http *http.Client
}

func NewClient() *Client {
	return &Client{
		http: &http.Client{Timeout: 15 * time.Second},
	}
}

// FetchWeek fetches all events for a given week and season type.
// seasonType: 2 = regular season, 3 = playoffs.
func (c *Client) FetchWeek(week, seasonYear, seasonType int) (*ScoreboardResponse, error) {
	url := fmt.Sprintf("%s?dates=%d&seasontype=%d&week=%d", baseURL, seasonYear, seasonType, week)

	resp, err := c.http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("espn fetch: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("espn returned status %d", resp.StatusCode)
	}

	var sb ScoreboardResponse
	if err := json.NewDecoder(resp.Body).Decode(&sb); err != nil {
		return nil, fmt.Errorf("espn decode: %w", err)
	}
	return &sb, nil
}

type ScoreboardResponse struct {
	Events []Event `json:"events"`
}

type Event struct {
	ID           string        `json:"id"`
	Date         string        `json:"date"`
	Competitions []Competition `json:"competitions"`
}

type Competition struct {
	Competitors []Competitor  `json:"competitors"`
	Odds        []Odds        `json:"odds"`
	Status      EventStatus   `json:"status"`
}

type Competitor struct {
	HomeAway string `json:"homeAway"` // "home" or "away"
	Team     Team   `json:"team"`
	Score    string `json:"score"`
}

type Team struct {
	Abbreviation string `json:"abbreviation"`
	DisplayName  string `json:"displayName"`
}

type Odds struct {
	Details string `json:"details"` // e.g. "KC -3.5"
}

type EventStatus struct {
	Type StatusType `json:"type"`
}

type StatusType struct {
	Name      string `json:"name"`      // "STATUS_SCHEDULED", "STATUS_IN_PROGRESS", "STATUS_FINAL"
	Completed bool   `json:"completed"` // true when game is final
}
