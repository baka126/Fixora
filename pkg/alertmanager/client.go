package alertmanager

import (
	"encoding/json"
	"fmt"
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
	resp, err := c.httpClient.Get(fmt.Sprintf("%s/api/v2/alerts", c.url))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var alerts []Alert
	if err := json.NewDecoder(resp.Body).Decode(&alerts); err != nil {
		return nil, err
	}

	return alerts, nil
}

func (c *Client) GetAlertsForPod(namespace, podName string) ([]Alert, error) {
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

	return filtered, nil
}
