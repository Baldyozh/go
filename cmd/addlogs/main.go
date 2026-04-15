package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"log-processor/internal/domain/entities"
	"log-processor/internal/infrastructure/kafka"
)

func main() {
	brokers := []string{"localhost:9092"}
	topic := "logs"
	messageCount := 5

	producer := kafka.NewProducer(brokers, topic)
	defer producer.Close()

	var wg sync.WaitGroup
	wg.Add(messageCount)

	ctx := context.Background()

	fmt.Printf("Adding %d test messages to Kafka topic '%s'\n", messageCount, topic)

	for i := 0; i < messageCount; i++ {
		iStr := strconv.Itoa(i)
		iUint32 := uint32(i)

		// Create ClickHouse log record
		record := &entities.ClickHouseLogRecord{
			LogID:         "log-" + iStr,
			Timestamp:     time.Now(),
			IntegrationID: "integration-1",
			RequestID:     "request-" + iStr,
			HTTPMethod:    "POST",
			Endpoint:      "/api/v1/users",
			RequestBody:   `{"user_id":` + iStr + `,"action":"create","passport":"1234567890","snils":"123-456-789 00"}`,
			ResponseBody:  `{"status":"success","id":` + iStr + `}`,
			DurationMs:    100 + iUint32,
			StatusCode:    200,
			Success:       true,
			ErrorMessage:  "",
			UserID:        &iUint32,
		}

		// Serialize record to JSON
		message, err := record.ToJSON()
		if err != nil {
			log.Printf("error serializing record %s: %v", iStr, err)
			wg.Done()
			continue
		}

		go func(msg []byte) {
			defer wg.Done()
			err := producer.WriteLog(ctx, entities.Log{
				TimeStamp: time.Now(),
				LogBody:   *record,
			})
			if err != nil {
				log.Printf("error writing message: %v", err)
				return
			}
			fmt.Printf("Message delivered at %s\n", time.Now().Format(time.RFC3339))
		}(message)
	}

	wg.Wait()
	fmt.Println("All messages delivered")
}
