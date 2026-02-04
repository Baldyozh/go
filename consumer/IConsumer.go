package consumer

import (
	"log-processor/entities"
)

type IConsumer interface {
	ReadLog() entities.Log
}
