package alertmanager

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

type Client struct {
	url        string
	httpClient *http.Client
}

type Alert struct {
	Labels       map[string]string `json:"labels"`
	Annotations  map[string]string `json:"annotations"`
	StartsAt     time.Time         `json:"startsAt"`
	EndsAt       time.Time         `json:"endsAt"`
	GeneratorURL string            `json:"generatorURL"`
	Status       struct {
		State       string   `json:"state"`
		SilencedBy  []string `json:"silencedBy"`
		InhibitedBy []string `json:"inhibitedBy"`
	} `json:"status"`
}

func New(url string) *Client {
	return &Client{
		url: url,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) GetAlerts() ([]Alert, error) {
	slog.Debug("Fetching alerts from Alertmanager", "url", c.url)
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/api/v2/alerts", c.url))
	if err != nil {
		slog.Error("Failed to connect to Alertmanager", "url", c.url, "error", err)
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		slog.Error("Alertmanager returned non-OK status", "url", c.url, "status_code", resp.StatusCode)
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var alerts []Alert
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		slog.Error("Failed to decode Alertmanager response", "url", c.url, "error", err)
		return nil, err
	}

	slog.Debug("Successfully retrieved alerts from Alertmanager", "count", len(alerts))
	return alerts, nil
}

func (c *Client) GetAlertsForPod(namespace, podName string) ([]Alert, error) {
	slog.Debug("Filtering Alertmanager alerts for specific pod", "ns", namespace, "pod", podName)
	// Filtering in memory for simplicity, though V2 API supports filters
	allAlerts, err := c.GetAlerts()
	if err != nil {
		return nil, err
	}

	var filtered []Alert
	for _, alert := range allAlerts {
		if alert.Labels["namespace"] == namespace && alert.Labels["pod"] == podName {
			filtered = append(filtered, alert)
		}
	}

	slog.Debug("Pod-specific alert filter complete", "ns", namespace, "pod", podName, "found_count", len(filtered))
	return filtered, nil
}
