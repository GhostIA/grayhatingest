package database_test

import (
	"context"
	"errors"
	"testing"

	"ingestor/database"

	"github.com/stretchr/testify/assert"
)

func TestMockDatabaseBehavior(t *testing.T) {
	// Initialize a mock database
	mockDB := database.NewMockMongoDB(true, true)
	// Test DoesDatabaseExist
	exists, err := mockDB.DoesDatabaseExist(context.Background(), "testDB")
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test DoesDocumentExist with no documents
	docExists, err := mockDB.DoesDocumentExist(context.Background(), "testCollection", nil)
	assert.NoError(t, err)
	assert.False(t, docExists)

	// Simulate a document existence
	mockDB.SetDocumentExists("testCollection", true)
	docExists, err = mockDB.DoesDocumentExist(context.Background(), "testCollection", nil)
	assert.NoError(t, err)
	assert.True(t, docExists)

	// Simulate an insert error
	mockDB.SetPredefinedInsertError(errors.New("mock insert error"))
	err = mockDB.InsertData(context.Background(), "testCollection", map[string]interface{}{"key": "value"})
	assert.Error(t, err)
	assert.Equal(t, "mock insert error", err.Error())
}
