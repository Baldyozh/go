package entities

import "time"

// Log represents a log message from Kafka
type Log struct {
	TimeStamp time.Time
	LogBody   ClickHouseLogRecord
}
