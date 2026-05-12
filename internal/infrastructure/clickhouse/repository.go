package clickhouse

import (
	"context"
	"fmt"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
	"github.com/Baldyozh/log-processor/internal/usecase/manage_logs"
	"github.com/ClickHouse/clickhouse-go/v2"
)

// QueryRepository handles read operations from ClickHouse
type QueryRepository struct {
	conn clickhouse.Conn
}

// NewQueryRepository creates a new query repository
func NewQueryRepository(conn clickhouse.Conn) *QueryRepository {
	return &QueryRepository{conn: conn}
}

// GetLogByID retrieves a single log by ID
func (r *QueryRepository) GetLogByID(ctx context.Context, logID string) (*entities.ClickHouseLogRecord, error) {
	query := `
		SELECT log_id, timestamp, integration_id, request_id, http_method, endpoint,
		       request_body, response_body, duration_ms, status_code, success, error_message, user_id
		FROM default.logs
		WHERE log_id = $1
		LIMIT 1
	`

	var record entities.ClickHouseLogRecord
	err := r.conn.QueryRow(ctx, query, logID).Scan(
		&record.LogID,
		&record.Timestamp,
		&record.IntegrationID,
		&record.RequestID,
		&record.HTTPMethod,
		&record.Endpoint,
		&record.RequestBody,
		&record.ResponseBody,
		&record.DurationMs,
		&record.StatusCode,
		&record.Success,
		&record.ErrorMessage,
		&record.UserID,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get log by ID: %w", err)
	}

	return &record, nil
}

// GetLogsByRequestID retrieves logs by request ID
func (r *QueryRepository) GetLogsByRequestID(ctx context.Context, requestID string) ([]entities.ClickHouseLogRecord, error) {
	query := `
		SELECT log_id, timestamp, integration_id, request_id, http_method, endpoint,
		       request_body, response_body, duration_ms, status_code, success, error_message, user_id
		FROM default.logs
		WHERE request_id = $1
		ORDER BY timestamp ASC
	`

	return r.queryLogs(ctx, query, requestID)
}

// QueryLogs retrieves logs based on filter criteria
func (r *QueryRepository) QueryLogs(ctx context.Context, filter manage_logs.LogFilter) ([]entities.ClickHouseLogRecord, error) {
	query := `
		SELECT log_id, timestamp, integration_id, request_id, http_method, endpoint,
		       request_body, response_body, duration_ms, status_code, success, error_message, user_id
		FROM default.logs
		WHERE 1=1
	`
	args := make([]interface{}, 0)
	argIndex := 1

	if filter.RequestID != "" {
		query += fmt.Sprintf(" AND request_id = $%d", argIndex)
		args = append(args, filter.RequestID)
		argIndex++
	}

	if filter.IntegrationID != "" {
		query += fmt.Sprintf(" AND integration_id = $%d", argIndex)
		args = append(args, filter.IntegrationID)
		argIndex++
	}

	if filter.StartTime != nil {
		query += fmt.Sprintf(" AND timestamp >= $%d", argIndex)
		args = append(args, *filter.StartTime)
		argIndex++
	}

	if filter.EndTime != nil {
		query += fmt.Sprintf(" AND timestamp <= $%d", argIndex)
		args = append(args, *filter.EndTime)
		argIndex++
	}

	if filter.StatusCode != nil {
		query += fmt.Sprintf(" AND status_code = $%d", argIndex)
		args = append(args, *filter.StatusCode)
		argIndex++
	}

	if filter.HTTPMethod != "" {
		query += fmt.Sprintf(" AND http_method = $%d", argIndex)
		args = append(args, filter.HTTPMethod)
		argIndex++
	}

	if filter.Endpoint != "" {
		query += fmt.Sprintf(" AND endpoint LIKE $%d", argIndex)
		args = append(args, "%"+filter.Endpoint+"%")
		argIndex++
	}

	if filter.UserID != nil {
		query += fmt.Sprintf(" AND user_id = $%d", argIndex)
		args = append(args, *filter.UserID)
		argIndex++
	}

	if filter.Success != nil {
		query += fmt.Sprintf(" AND success = $%d", argIndex)
		args = append(args, *filter.Success)
		argIndex++
	}

	query += " ORDER BY timestamp DESC"

	return r.queryLogs(ctx, query, args...)
}

// GetLogsStats returns statistics about logs
func (r *QueryRepository) GetLogsStats(ctx context.Context, filter manage_logs.LogFilter) (map[string]interface{}, error) {
	query := `
		SELECT
			count() as total_logs,
			countIf(success = true) as successful_logs,
			countIf(success = false) as failed_logs,
			avg(duration_ms) as avg_duration_ms,
			quantile(0.95)(duration_ms) as p95_duration_ms
		FROM default.logs
		WHERE 1=1
	`
	args := make([]interface{}, 0)
	argIndex := 1

	if filter.IntegrationID != "" {
		query += fmt.Sprintf(" AND integration_id = $%d", argIndex)
		args = append(args, filter.IntegrationID)
		argIndex++
	}

	if filter.StartTime != nil {
		query += fmt.Sprintf(" AND timestamp >= $%d", argIndex)
		args = append(args, *filter.StartTime)
		argIndex++
	}

	if filter.EndTime != nil {
		query += fmt.Sprintf(" AND timestamp <= $%d", argIndex)
		args = append(args, *filter.EndTime)
		argIndex++
	}

	var stats struct {
		TotalLogs      uint64  `ch:"total_logs"`
		SuccessfulLogs uint64  `ch:"successful_logs"`
		FailedLogs     uint64  `ch:"failed_logs"`
		AvgDurationMs  float64 `ch:"avg_duration_ms"`
		P95DurationMs  float64 `ch:"p95_duration_ms"`
	}

	err := r.conn.QueryRow(ctx, query, args...).Scan(
		&stats.TotalLogs,
		&stats.SuccessfulLogs,
		&stats.FailedLogs,
		&stats.AvgDurationMs,
		&stats.P95DurationMs,
	)

	if err != nil {
		return nil, fmt.Errorf("failed to get logs stats: %w", err)
	}

	return map[string]interface{}{
		"total_logs":       stats.TotalLogs,
		"successful_logs":  stats.SuccessfulLogs,
		"failed_logs":      stats.FailedLogs,
		"avg_duration_ms":  stats.AvgDurationMs,
		"p95_duration_ms":  stats.P95DurationMs,
	}, nil
}

// queryLogs is a helper method to execute log queries
func (r *QueryRepository) queryLogs(ctx context.Context, query string, args ...interface{}) ([]entities.ClickHouseLogRecord, error) {
	rows, err := r.conn.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var records []entities.ClickHouseLogRecord
	for rows.Next() {
		var record entities.ClickHouseLogRecord
		err := rows.Scan(
			&record.LogID,
			&record.Timestamp,
			&record.IntegrationID,
			&record.RequestID,
			&record.HTTPMethod,
			&record.Endpoint,
			&record.RequestBody,
			&record.ResponseBody,
			&record.DurationMs,
			&record.StatusCode,
			&record.Success,
			&record.ErrorMessage,
			&record.UserID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log record: %w", err)
		}
		records = append(records, record)
	}

	return records, nil
}
