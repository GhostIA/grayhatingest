package ingestor

import (
	"context"
	"ingestor/database"
	"ingestor/fetcher"
	"io"
	"sync"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/mock"
)

// Mock for database.MongoDBInterface
type MockMongoDB struct {
	mock.Mock
}

// DoesCollectionExist implements database.MongoDBInterface.
func (m *MockMongoDB) DoesCollectionExist(ctx context.Context, collectionName string) (bool, error) {
	panic("unimplemented")
}

// DoesDatabaseExist implements database.MongoDBInterface.
func (m *MockMongoDB) DoesDatabaseExist(ctx context.Context, dbName string) (bool, error) {
	panic("unimplemented")
}

func (m *MockMongoDB) DoesDocumentExist(ctx context.Context, collection string, filter map[string]interface{}) (bool, error) {
	args := m.Called(ctx, collection, filter)
	return args.Bool(0), args.Error(1)
}

func (m *MockMongoDB) InsertData(ctx context.Context, collection string, data interface{}) error {
	args := m.Called(ctx, collection, data)
	return args.Error(0)
}

var _ database.MongoDBInterface = (*MockMongoDB)(nil)

func TestIngestor_StartWorkers(t *testing.T) {
	// Mock Logger
	logger := logrus.New()
	logger.SetOutput(io.Discard) // Suppress log output during tests

	// Mock MongoDB
	mockDB := new(MockMongoDB)

	// Mock behavior
	mockDB.On("DoesDocumentExist", mock.Anything, "test_collection", mock.MatchedBy(func(filter map[string]interface{}) bool {
		return filter["url"] == "http://example.com/exists"
	})).Return(true, nil)

	mockDB.On("DoesDocumentExist", mock.Anything, "test_collection", mock.MatchedBy(func(filter map[string]interface{}) bool {
		return filter["url"] == "http://example.com/new"
	})).Return(false, nil)

	mockDB.On("InsertData", mock.Anything, "test_collection", mock.Anything).Return(nil)

	// Create an Ingestor
	ingestor := NewIngestor(mockDB, logger)

	// Test data channel
	dataChan := make(chan fetcher.FilteredData, 2)
	dataChan <- fetcher.FilteredData{URL: "http://example.com/exists", FileCount: 10}
	dataChan <- fetcher.FilteredData{URL: "http://example.com/new", FileCount: 5}
	close(dataChan)

	// Context and WaitGroup
	ctx := context.Background()
	wg := &sync.WaitGroup{}

	// Start Workers
	ingestor.StartWorkers(ctx, dataChan, "test_collection", 2, wg)

	// Wait for workers to complete
	wg.Wait()

	// Assertions
	mockDB.AssertCalled(t, "DoesDocumentExist", mock.Anything, "test_collection", mock.MatchedBy(func(filter map[string]interface{}) bool {
		return filter["url"] == "http://example.com/exists"
	}))
	mockDB.AssertCalled(t, "DoesDocumentExist", mock.Anything, "test_collection", mock.MatchedBy(func(filter map[string]interface{}) bool {
		return filter["url"] == "http://example.com/new"
	}))
	mockDB.AssertCalled(t, "InsertData", mock.Anything, "test_collection", fetcher.FilteredData{URL: "http://example.com/new", FileCount: 5})
	mockDB.AssertNotCalled(t, "InsertData", mock.Anything, "test_collection", fetcher.FilteredData{URL: "http://example.com/exists", FileCount: 10})
}
