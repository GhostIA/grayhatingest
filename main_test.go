package main

import (
	"context"
	"ingestor/database"
	"ingestor/fetcher"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
)

// MockFetcher implements fetcher.FetcherInterface for testing
type MockFetcher struct{}

func (m *MockFetcher) FetchAndFilterData(maxRetries int, dataChan chan<- fetcher.FilteredData) {
	defer close(dataChan)

	// Simulate sending filtered data
	dataChan <- fetcher.FilteredData{URL: "http://example.com/bucket1", FileCount: 10}
	dataChan <- fetcher.FilteredData{URL: "http://example.com/bucket2", FileCount: 5}
}

var _ fetcher.FetcherInterface = (*MockFetcher)(nil)

func TestProcessEnvironment(t *testing.T) {
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	// Initialize MockMongoDB
	mockDB := database.NewMockMongoDB(true, true)
	mockFetcher := &MockFetcher{}
	// Configure mock behavior
	mockDB.SetDocumentExists("filteredData", false) // Simulate no documents exist
	mockDB.SetPredefinedInsertError(nil)            // No insert errors

	// Test parameters
	ctx := context.Background()
	wg := &sync.WaitGroup{}
	apiEndpoint := "http://mock-api.com"
	headers := map[string]string{"Authorization": "Bearer mock-token"}
	mongoURI := "mongodb://mock-mongo-uri"
	baseDBName := "test_db"
	retryCount := 3
	workerCount := 5
	start := 0
	limit := 10
	env := AWS
	timestamp := "20231218T123456"
	order := "fileCount"
	direction := "desc"

	// Add to WaitGroup
	wg.Add(1)

	// MockFetcher for testing

	// Call processEnvironment with mock dependencies
	processEnvironment(ctx, wg, log, apiEndpoint, headers, mongoURI, baseDBName, retryCount, workerCount, start, limit, env, timestamp, order, direction,
		func(apiEndpoint string, headers map[string]string, start, limit int, env Environment) fetcher.FetcherInterface {
			return mockFetcher
		},
		func(mongoURI, dbName string) (database.MongoDBInterface, error) {
			mockDB := database.NewMockMongoDB(true, true)
			mockDB.SetDocumentExists("filteredData", false)
			return mockDB, nil
		},
	)
	t.Log("Test completed successfully with MockMongoDB and MockFetcher")
}

func TestParallelEnvironments(t *testing.T) {
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	// Initialize MockMongoDB
	mockDB := database.NewMockMongoDB(true, true)

	// Configure mock behavior
	mockDB.SetDocumentExists("filteredData", false) // Simulate no documents exist
	mockDB.SetPredefinedInsertError(nil)            // No insert errors

	// Test parameters
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	wg := &sync.WaitGroup{}
	apiEndpoint := "http://mock-api.com"
	headers := map[string]string{"Authorization": "Bearer mock-token"}
	mongoURI := "mongodb://mock-mongo-uri"
	baseDBName := "test_db"
	retryCount := 3
	workerCount := 5
	start := 0
	limit := 10
	timestamp := "20231218T123456"
	order := "fileCount"
	direction := "asc"

	// MockFetcher for testing
	mockFetcher := &MockFetcher{}

	// Process each environment in parallel
	for _, env := range GetAllEnvironments() {
		wg.Add(1)
		processEnvironment(ctx, wg, log, apiEndpoint, headers, mongoURI, baseDBName, retryCount, workerCount, start, limit, env, timestamp, order, direction,
			func(apiEndpoint string, headers map[string]string, start, limit int, env Environment) fetcher.FetcherInterface {
				return mockFetcher
			},
			func(mongoURI, dbName string) (database.MongoDBInterface, error) {
				mockDB := database.NewMockMongoDB(true, true)
				mockDB.SetDocumentExists("filteredData", false)
				return mockDB, nil
			},
		)
		t.Log("Test completed successfully with MockMongoDB and MockFetcher")
	}
}
