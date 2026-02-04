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
var messageAmount = 100

func AddLogsToKafka() {
	writer := kafka.NewWriter(kafka.WriterConfig{
		Brokers: []string{"localhost:9092"},
		Topic:   "logs",
	})
	defer writer.Close()

	wg.Add(messageAmount)
	for i := 0; i < messageAmount; i++ {
		iStr := strconv.Itoa(i)
		msg := []byte(`{status: 200,` +
			`partner_id=10` +
			`passport=1232133312,` +
			`snils=123124124123123,` +
			`otherdata="asdasdasdasdasdasddfgadfadsdfasdf}` +
			`id=` + iStr)

		go WriteLogsToKafka(writer, msg)
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
