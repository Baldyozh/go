package test

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"sync"

	"github.com/segmentio/kafka-go"
)

var wg sync.WaitGroup
var messageAmount = 5

func AddLogsToKafka() {
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "logs",
	})
	defer writer.Close()

	wg.Add(messageAmount)

	for i := 0; i < messageAmount; i++ {
		iStr := strconv.Itoa(i)

		msg := "{\n" +
			`  "email":` + strconv.Quote("root@example.com") + ",\n" +
			`  "user": {` +
			`"email":` + strconv.Quote("alice@example.com") + ", " +
			`"id":` + iStr + `, ` +
			`"profile": {"contact":{"email":` + strconv.Quote("deep@example.com") + `}}` +
			"},\n" +
			`  "companies": [` +
			`{"email":` + strconv.Quote("c1@example.com") + `}, ` +
			`{"sens": {"passport":` + strconv.Quote("1233123233") + `}}` +
			"]\n" +
			"}"

		go WriteLogsToKafka(writer, []byte(msg))

	}
	wg.Wait()
}

func WriteLogsToKafka(writer *kafka.Writer, message []byte) {
	defer wg.Done()
	ctx := context.Background()

	err := writer.WriteMessages(ctx, kafka.Message{
		Value: message,
	})
	if err != nil {
		log.Println("send error", err)
		return
	}
	fmt.Println("message delivered")

}
