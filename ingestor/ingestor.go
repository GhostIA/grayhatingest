package ingestor

import (
	"context"
	"ingestor/database"
	"ingestor/fetcher"
	"sync"

	"github.com/sirupsen/logrus"
)

type Ingestor struct {
	DB     database.MongoDBInterface
	Logger *logrus.Logger
}

func NewIngestor(db database.MongoDBInterface, logger *logrus.Logger) *Ingestor {
	return &Ingestor{
		DB:     db,
		Logger: logger,
	}
}

func (i *Ingestor) StartWorkers(ctx context.Context, dataChan <-chan fetcher.FilteredData, collection string, workerCount int, wg *sync.WaitGroup) {
	for workerID := 0; workerID < workerCount; workerID++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for data := range dataChan {
				exists, err := i.DB.DoesDocumentExist(ctx, collection, map[string]interface{}{
					"url": data.URL,
				})
				if err != nil {
					i.Logger.Warnf("Worker %d failed to check existence: %v", workerID, err)
					continue
				}
				if exists {
					i.Logger.Infof("Worker %d skipping duplicate: %+v", workerID, data)
					continue
				}
				if err := i.DB.InsertData(ctx, collection, data); err != nil {
					i.Logger.Warnf("Worker %d failed to insert: %v", workerID, err)
				}
			}
		}(workerID)
	}
}
