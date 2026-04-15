package clickhouse

import (
	"context"
	"fmt"
	"log"
	"log-processor/internal/domain/entities"

	"github.com/ClickHouse/clickhouse-go/v2"
)

const batchSize = 10

type DBConfig struct {
	Address  string
	Port     int
	Login    string
	Password string
	Database string
}

type LogDBClient interface {
	Close()
	EnsureTableExists() error
	InsertBatch(records []entities.ClickHouseLogRecord) error
}

type Client struct {
	conn clickhouse.Conn
}

func New(config DBConfig) *Client {
	ctx := context.Background()
	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", config.Address, config.Port)},
		Auth: clickhouse.Auth{
			Username: config.Login,
			Password: config.Password,
		},
	})
	if err != nil {
		log.Fatalf("Failed to connect to ClickHouse: %v", err)
	}

	if err := conn.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping ClickHouse: %v", err)
	}

	client := &Client{conn: conn}

	if err := client.EnsureTableExists(); err != nil {
		log.Fatalf("Failed to ensure table exists: %v", err)
	}

	return client
}

func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *Client) EnsureTableExists() error {
	query := `
		CREATE TABLE IF NOT EXISTS default.logs (
			log_id         String,
			timestamp      DateTime64(3),
			integration_id String,
			request_id     String,
			http_method    String,
			endpoint       String,
			request_body   String,
			response_body  String,
			duration_ms    UInt32,
			status_code    UInt16,
			success        Bool,
			error_message  String,
			user_id        Nullable(UInt32)
		) ENGINE = MergeTree()
		ORDER BY (timestamp, log_id)
	`
	return c.conn.Exec(context.Background(), query)
}

func (c *Client) InsertBatch(records []entities.ClickHouseLogRecord) error {
	if len(records) == 0 {
		return nil
	}

	batch, err := c.conn.PrepareBatch(context.Background(), "INSERT INTO default.logs (log_id, timestamp, integration_id, request_id, http_method, endpoint, request_body, response_body, duration_ms, status_code, success, error_message, user_id)")
	if err != nil {
		return fmt.Errorf("failed to prepare batch: %w", err)
	}

	for _, record := range records {
		if err := batch.Append(
			record.LogID,
			record.Timestamp,
			record.IntegrationID,
			record.RequestID,
			record.HTTPMethod,
			record.Endpoint,
			record.RequestBody,
			record.ResponseBody,
			record.DurationMs,
			record.StatusCode,
			record.Success,
			record.ErrorMessage,
			record.UserID,
		); err != nil {
			return fmt.Errorf("failed to append record to batch: %w", err)
		}
	}

	return batch.Send()
}
