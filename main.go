package main

import (
	"context"
	"fmt"
	"ingestor/database"
	"ingestor/fetcher"
	"ingestor/ingestor"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/sirupsen/logrus"
)

// Enum-like type for environment
type Environment string

// Enum values
const (
	AWS   Environment = "aws"
	Azure Environment = "azure"
	GCP   Environment = "gcp"
	DOS   Environment = "dos"
)

// Function to return all enum values
func GetAllEnvironments() []Environment {
	return []Environment{AWS, Azure, GCP, DOS}
}

// Config struct to store dbName
type Config struct {
	baseDBName string
	env        Environment
	timestamp  string
}

// Method to retrieve the full database name
func (c Config) GetDatabaseName() string {
	return fmt.Sprintf("%s_%s_%s", c.baseDBName, c.env, c.timestamp)
}

func processEnvironment(
	ctx context.Context,
	wg *sync.WaitGroup,
	log *logrus.Logger,
	apiEndpoint string,
	headers map[string]string,
	mongoURI, baseDBName string,
	retryCount, workerCount, start, limit int,
	env Environment,
	direction, order, timestamp string,
	createFetcher func(apiEndpoint string, headers map[string]string, start, limit int, env Environment) fetcher.FetcherInterface,
	createDB func(mongoURI, dbName string) (database.MongoDBInterface, error),
) {
	defer wg.Done()

	log.Infof("Processing environment: %s", env)

	// Create a unique database name
	config := Config{
		baseDBName: baseDBName,
		env:        env,
		timestamp:  timestamp,
	}
	httpClient := &http.Client{
		Timeout: 10 * time.Second, // Set a timeout for API requests
		Transport: &http.Transport{
			MaxIdleConns:    10,
			IdleConnTimeout: 30 * time.Second,
		},
	}

	dbName := config.GetDatabaseName()
	log.Infof("Trying to create db database %s", dbName)
	// Initialize MongoDB with environment-specific and timestamped dbName
	mongoDB, err := database.NewMongoDB(mongoURI, dbName)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB for environment %s: %v", env, err)
	}

	// Check database and collection existence
	dbExists, err := mongoDB.DoesDatabaseExist(ctx, dbName)
	if err != nil {
		log.Fatalf("Failed to check database existence for environment %s: %v", env, err)
	}
	if !dbExists {
		log.Infof("Database '%s' does not exist for environment %s. It will be created automatically during insertion.", dbName, env)
	}

	collectionExists, err := mongoDB.DoesCollectionExist(ctx, "filteredData")
	if err != nil {
		log.Fatalf("Failed to check collection existence for environment %s: %v", env, err)
	}
	if !collectionExists {
		log.Infof("Collection 'filteredData' does not exist for environment %s. It will be created automatically during insertion.", env)
	}

	// Initialize Fetcher and Ingestor

	fetcherInstance := fetcher.NewFetcher(apiEndpoint, headers, log, start, limit, string(env), order, direction, httpClient)
	dataChan := make(chan fetcher.FilteredData, 100)
	ingestor := ingestor.NewIngestor(mongoDB, log)

	// Start Fetcher
	var localWg sync.WaitGroup
	localWg.Add(1)
	go func() {
		defer localWg.Done()
		fetcherInstance.FetchAndFilterData(retryCount, dataChan) // Closes dataChan when done
	}()

	// Start Workers
	ingestor.StartWorkers(ctx, dataChan, "filteredData", workerCount, &localWg)

	// Wait for local workers to finish
	localWg.Wait()
	log.Infof("Processing for environment '%s' completed.", env)
}

func main() {
	// Initialize logger
	log := logrus.New()
	log.SetFormatter(&logrus.TextFormatter{FullTimestamp: true})

	// Load configuration
	apiEndpoint := os.Getenv("API_ENDPOINT")
	headers := map[string]string{
		"Content-Type":  os.Getenv("API_CONTENT_TYPE"),
		"Authorization": fmt.Sprintf("Bearer %s", os.Getenv("API_AUTHORIZATION_TOKEN")),
	}
	mongoURI := os.Getenv("MONGODB_URI")
	baseDBName := os.Getenv("DATABASE_NAME")
	retryCount, _ := strconv.Atoi(os.Getenv("RETRY_COUNT"))
	workerCount, _ := strconv.Atoi(os.Getenv("WORKER_COUNT"))
	start := 0 // Start page
	limit := 10
	order := "fileCount"
	direction := "asc"
	client := &http.Client{}
	// Generate a timestamp for the database name
	timestamp := time.Now().Format("20060102T150405") // Format: YYYYMMDDTHHMMSS

	// Context for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Graceful shutdown setup
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt, syscall.SIGTERM)

	// WaitGroup to manage goroutines
	var wg sync.WaitGroup

	// Define the fetcher factory
	fetcherFactory := func(apiEndpoint string, headers map[string]string, start, limit int, env Environment) fetcher.FetcherInterface {
		return fetcher.NewFetcher(apiEndpoint, headers, log, start, limit, string(env), order, direction, client)
	}

	// Define the database factory
	dbFactory := func(mongoURI, dbName string) (database.MongoDBInterface, error) {
		return database.NewMongoDB(mongoURI, dbName)
	}

	// Iterate through each environment and process in parallel
	for _, env := range GetAllEnvironments() {
		wg.Add(1)
		go processEnvironment(ctx, &wg, log, apiEndpoint, headers, mongoURI, baseDBName, retryCount, workerCount, start, limit, env, direction, order, timestamp, fetcherFactory, dbFactory)
	}

	// Handle shutdown
	go func() {
		<-signalChan
		log.Warn("Termination signal received. Shutting down...")
		cancel()
	}()

	// Wait for all goroutines to complete
	wg.Wait()
	log.Info("All tasks completed. Exiting.")
}
