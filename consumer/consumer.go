package consumer

import (
	"context"
	"fmt"
	"log"
	"log-processor/entities"

	"github.com/segmentio/kafka-go"
)

type KafkaConsumer struct {
	Reader *kafka.Reader
	DataCh chan []byte
}

func NewKafkaConsumer(KafkaReader kafka.Reader) *KafkaConsumer {

	consumer := new(KafkaConsumer)
	consumer.DataCh = make(chan []byte)
	consumer.Reader = &KafkaReader
	return consumer
}
func (consumer *KafkaConsumer) ReadLog() (entities.Log, error) {

	msg, err := consumer.Reader.ReadMessage(context.Background())
	if err != nil {
		log.Fatal("Ошибка при получении:", err)
		return entities.Log{}, err
	}

	fmt.Println(string(msg.Value))
	return entities.Log{
		TimeStamp: msg.Time,
		LogBody:   string(msg.Value),
	}, nil

}
