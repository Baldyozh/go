package consumer

import (
	"context"
	"fmt"
	"log"
	"log-processor/entities"

	"github.com/segmentio/kafka-go"
)

type KafkaConsumer struct {
	reader *kafka.Reader
}

func NewKafkaConsumer(KafkaReader kafka.Reader) *KafkaConsumer {

	consumer := new(KafkaConsumer)

	consumer.reader = &KafkaReader
	return consumer
}
func (consumer *KafkaConsumer) ReadLog() entities.Log {

	msg, err := consumer.reader.ReadMessage(context.Background())
	if err != nil {
		log.Fatal("Ошибка при получении:", err)
	}

	fmt.Println(string(msg.Value))
	return entities.Log{
		TimeStamp: msg.Time,
		LogBody:   string(msg.Value),
	}

}
