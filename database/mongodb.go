package database

import (
	"context"
	"strings"
	"sync"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoDBInterface interface {
	InsertData(ctx context.Context, collection string, data interface{}) error
	DoesDatabaseExist(ctx context.Context, dbName string) (bool, error)
	DoesCollectionExist(ctx context.Context, collectionName string) (bool, error)
	DoesDocumentExist(ctx context.Context, collection string, filter map[string]interface{}) (bool, error)
}

type MongoDB struct {
	client   *mongo.Client
	database *mongo.Database
}

func NewMongoDB(uri, dbName string) (MongoDBInterface, error) {
	if isTestURI(uri) {
		return NewMockMongoDB(true, true), nil // Return mock database for test URIs
	}

	clientOptions := options.Client().ApplyURI(uri)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		return nil, err
	}
	return &MongoDB{client: client, database: client.Database(dbName)}, nil
}

// isTestURI checks if the provided URI is for testing
func isTestURI(uri string) bool {
	return strings.Contains(uri, "test") || strings.Contains(uri, "mock")
}

func (db *MongoDB) InsertData(ctx context.Context, collection string, data interface{}) error {
	_, err := db.database.Collection(collection).InsertOne(ctx, data)
	return err
}

func (db *MongoDB) DoesDatabaseExist(ctx context.Context, dbName string) (bool, error) {
	dbNames, err := db.client.ListDatabaseNames(ctx, bson.M{})
	if err != nil {
		return false, err
	}
	for _, name := range dbNames {
		if name == dbName {
			return true, nil
		}
	}
	return false, nil
}

func (db *MongoDB) DoesCollectionExist(ctx context.Context, collectionName string) (bool, error) {
	collectionNames, err := db.database.ListCollectionNames(ctx, bson.M{})
	if err != nil {
		return false, err
	}
	for _, name := range collectionNames {
		if name == collectionName {
			return true, nil
		}
	}
	return false, nil
}

func (db *MongoDB) DoesDocumentExist(ctx context.Context, collection string, filter map[string]interface{}) (bool, error) {
	count, err := db.database.Collection(collection).CountDocuments(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

type MockMongoDB struct {
	mu                  sync.Mutex
	databaseExists      bool
	collectionExists    bool
	documentsExist      map[string]bool
	predefinedInsertErr error
}

var _ MongoDBInterface = (*MockMongoDB)(nil)

func NewMockMongoDB(databaseExists, collectionExists bool) *MockMongoDB {
	return &MockMongoDB{
		databaseExists:      databaseExists,
		collectionExists:    collectionExists,
		documentsExist:      make(map[string]bool),
		predefinedInsertErr: nil,
	}
}

func (m *MockMongoDB) InsertData(ctx context.Context, collection string, data interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.predefinedInsertErr // Simulate an insert error if configured
}

func (m *MockMongoDB) DoesDatabaseExist(ctx context.Context, dbName string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.databaseExists, nil
}

func (m *MockMongoDB) DoesCollectionExist(ctx context.Context, collectionName string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.collectionExists, nil
}

func (m *MockMongoDB) DoesDocumentExist(ctx context.Context, collection string, filter map[string]interface{}) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := collection // Simulate a key for simplicity; you can expand this
	exists, ok := m.documentsExist[key]
	if !ok {
		return false, nil
	}
	return exists, nil
}

// Helper methods for configuring mock behavior
func (m *MockMongoDB) SetDocumentExists(collection string, exists bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.documentsExist[collection] = exists
}

func (m *MockMongoDB) SetPredefinedInsertError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.predefinedInsertErr = err
}
