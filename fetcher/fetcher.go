package fetcher

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

type FetcherInterface interface {
	FetchAndFilterData(maxRetries int, dataChan chan<- FilteredData)
}

// Ensure Fetcher implements FetcherInterface
var _ FetcherInterface = (*Fetcher)(nil)

type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

type RawData struct {
	Query struct {
		Keywords  string `json:"keywords"`
		Exact     bool   `json:"exact"`
		Start     int    `json:"start"`
		Limit     int    `json:"limit"`
		Type      string `json:"type"`
		Order     string `json:"order"`
		Direction string `json:"direction"`
	} `json:"query"`
	Meta struct {
		Results int `json:"results"`
	} `json:"meta"`
	Buckets []struct {
		ID        int    `json:"id"`
		Bucket    string `json:"bucket"`
		FileCount int    `json:"fileCount"`
		Type      string `json:"type"`
	} `json:"buckets"`
}

type FilteredData struct {
	URL       string `json:"url"`
	FileCount int    `json:"fileCount"`
}

// Fetcher with a configurable HTTP client for testability
type Fetcher struct {
	APIEndpoint string
	Headers     map[string]string
	Logger      *logrus.Logger
	Start       int
	Limit       int
	Env         string
	Order       string
	Direction   string
	Client      HTTPClient
}

func NewFetcher(
	apiEndpoint string,
	headers map[string]string,
	logger *logrus.Logger,
	start, limit int,
	env, order, direction string,
	client HTTPClient, // Allow injecting a custom HTTP client
) *Fetcher {
	// Use default HTTP client if none is provided
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &Fetcher{
		APIEndpoint: apiEndpoint,
		Headers:     headers,
		Logger:      logger,
		Start:       start,
		Limit:       limit,
		Env:         env,
		Order:       order,
		Direction:   direction,
		Client:      &http.Client{Timeout: 10 * time.Second}, // Default HTTP client
	}
}

// FetchData fetches raw JSON data from the API
func (f *Fetcher) FetchData() ([]byte, error) {
	// Construct the API URL
	url := fmt.Sprintf("%s?type=%s&start=%d&limit=%d&order=%s&direction=%s",
		f.APIEndpoint, f.Env, f.Start, f.Limit, f.Order, f.Direction)
	f.Logger.Infof("Fetching data from: %s", url)

	// Create the HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers to the request
	for key, value := range f.Headers {
		req.Header.Set(key, value)
	}

	// Send the request
	resp, err := f.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch data: %w", err)
	}
	defer resp.Body.Close()

	// Check for non-200 status codes
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	return body, nil
}

// ParseJSON parses raw JSON into FilteredData
func (f *Fetcher) ParseJSON(body []byte) ([]FilteredData, error) {
	var rawData RawData
	if err := json.Unmarshal(body, &rawData); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	// Transform RawData into FilteredData
	var filteredData []FilteredData
	for _, bucket := range rawData.Buckets {
		filteredData = append(filteredData, FilteredData{
			URL:       bucket.Bucket,
			FileCount: bucket.FileCount,
		})
	}

	return filteredData, nil
}

func (f *Fetcher) FetchAndFilterData(maxRetries int, dataChan chan<- FilteredData) {
	defer close(dataChan)

	for {
		// Fetch raw data
		body, err := f.FetchData()
		if err != nil {
			f.Logger.Warnf("Error fetching data: %v", err)
			if maxRetries > 0 {
				time.Sleep(2 * time.Second)
				maxRetries--
				continue
			}
			return
		}

		// Parse JSON data
		filteredData, err := f.ParseJSON(body)
		if err != nil {
			f.Logger.Warnf("Error parsing data: %v", err)
			return
		}

		// Send data to the channel
		for _, data := range filteredData {
			dataChan <- data
		}

		// Break if fewer results than the limit
		if len(filteredData) < f.Limit {
			break
		}

		// Update pagination
		f.Start += f.Limit
		time.Sleep(1 * time.Second)
	}
}
