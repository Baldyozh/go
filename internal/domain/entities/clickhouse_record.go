package entities

import (
	"encoding/json"
	"time"
)

// ClickHouseLogRecord represents a log record ready for insertion into ClickHouse
type ClickHouseLogRecord struct {
	LogID         string    `ch:"log_id" json:"log_id"`
	Timestamp     time.Time `ch:"timestamp" json:"timestamp"`
	IntegrationID string    `ch:"integration_id" json:"integration_id"`
	RequestID     string    `ch:"request_id" json:"request_id"`
	HTTPMethod    string    `ch:"http_method" json:"http_method"`
	Endpoint      string    `ch:"endpoint" json:"endpoint"`
	RequestBody   string    `ch:"request_body" json:"request_body"`
	ResponseBody  string    `ch:"response_body" json:"response_body"`
	DurationMs    uint32    `ch:"duration_ms" json:"duration_ms"`
	StatusCode    uint16    `ch:"status_code" json:"status_code"`
	Success       bool      `ch:"success" json:"success"`
	ErrorMessage  string    `ch:"error_message" json:"error_message"`
	UserID        *uint32   `ch:"user_id" json:"user_id,omitempty"`
}

// ToJSON serializes the record to JSON
func (r *ClickHouseLogRecord) ToJSON() ([]byte, error) {
	return json.Marshal(r)
}

// ClickHouseLogRecordBatch represents a batch of log records for bulk insertion
type ClickHouseLogRecordBatch struct {
	Records []*ClickHouseLogRecord
}

// NewClickHouseLogRecordBatch creates a new batch
func NewClickHouseLogRecordBatch() *ClickHouseLogRecordBatch {
	return &ClickHouseLogRecordBatch{
		Records: make([]*ClickHouseLogRecord, 0),
	}
}

// Add adds a record to the batch
func (b *ClickHouseLogRecordBatch) Add(record *ClickHouseLogRecord) {
	b.Records = append(b.Records, record)
}

// Len returns the number of records in the batch
func (b *ClickHouseLogRecordBatch) Len() int {
	return len(b.Records)
}

// Clear clears all records in the batch
func (b *ClickHouseLogRecordBatch) Clear() {
	b.Records = b.Records[:0]
}
