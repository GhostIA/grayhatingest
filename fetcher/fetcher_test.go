package fetcher_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"ingestor/fetcher"

	"github.com/sirupsen/logrus"
)

func TestFetchAndFilterData(t *testing.T) {
	// Mock HTTP server
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"buckets": [{"bucket": "http://example.com/bucket1", "fileCount": 10}, {"bucket": "http://example.com/bucket2", "fileCount": 5}]}`))
	}))
	defer mockServer.Close()

	// Mock Logger
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	// Create Fetcher with mock HTTP client
	client := mockServer.Client()
	f := fetcher.NewFetcher(mockServer.URL, map[string]string{"Authorization": "Bearer mock-token"}, log, 0, 10, "test", "fileCount", "dsc", client)

	// Data channel
	dataChan := make(chan fetcher.FilteredData, 10)
	go f.FetchAndFilterData(3, dataChan)

	// Collect results
	results := []fetcher.FilteredData{}
	for data := range dataChan {
		results = append(results, data)
	}

	// Assertions
	if len(results) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(results))
	}
	if results[0].URL != "http://example.com/bucket1" || results[1].URL != "http://example.com/bucket2" {
		t.Fatalf("Unexpected results: %+v", results)
	}
}
