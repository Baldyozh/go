package manage_logs

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
)

// LogReader is the interface for reading logs from storage
type LogReader interface {
	GetLogByID(ctx context.Context, logID string) (*entities.ClickHouseLogRecord, error)
	GetLogsByRequestID(ctx context.Context, requestID string) ([]entities.ClickHouseLogRecord, error)
	QueryLogs(ctx context.Context, filter LogFilter) ([]entities.ClickHouseLogRecord, error)
	GetLogsStats(ctx context.Context, filter LogFilter) (map[string]interface{}, error)
}

// LogEncrypter is the interface for decrypting log data
type LogEncrypter interface {
	DecryptFields(data []byte, fields []string) ([]byte, error)
}

// AuthPermissionChecker is the interface for checking permissions
type AuthPermissionChecker interface {
	HasPermission(ctx context.Context, userID int, permissionName string) (bool, error)
	LogDecryptionRequest(ctx context.Context, userID int, logID string, reason string) error
}

// LogFilter represents filter parameters
type LogFilter struct {
	RequestID      string
	IntegrationID  string
	StartTime     *time.Time
	EndTime       *time.Time
	StatusCode    *uint16
	HTTPMethod    string
	Endpoint      string
	UserID        *uint32
	Success       *bool
}

// LogService handles business logic for log management
type LogService struct {
	logReader      LogReader
	encrypter      LogEncrypter
	authChecker    AuthPermissionChecker
	sensitiveFields []string
}

// NewLogService creates a new log service
func NewLogService(
	logReader LogReader,
	encrypter LogEncrypter,
	authChecker AuthPermissionChecker,
	sensitiveFields []string,
) *LogService {
	return &LogService{
		logReader:      logReader,
		encrypter:      encrypter,
		authChecker:    authChecker,
		sensitiveFields: sensitiveFields,
	}
}

// GetLogByID retrieves a log by ID with optional decryption
func (s *LogService) GetLogByID(ctx context.Context, userID int, logID string, decrypt bool, reason string) (*entities.ClickHouseLogRecord, error) {
	// Check read permission
	hasPermission, err := s.authChecker.HasPermission(ctx, userID, "logs:read")
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}
	if !hasPermission {
		return nil, fmt.Errorf("permission denied: logs:read required")
	}

	// Get log
	log, err := s.logReader.GetLogByID(ctx, logID)
	if err != nil {
		return nil, fmt.Errorf("failed to get log: %w", err)
	}

	// Decrypt if requested and user has permission
	if decrypt {
		hasDecryptPermission, err := s.authChecker.HasPermission(ctx, userID, "logs:decrypt")
		if err != nil {
			return nil, fmt.Errorf("failed to check decrypt permission: %w", err)
		}
		if !hasDecryptPermission {
			return nil, fmt.Errorf("permission denied: logs:decrypt required")
		}

		// Log decryption request
		if err := s.authChecker.LogDecryptionRequest(ctx, userID, logID, reason); err != nil {
			return nil, fmt.Errorf("failed to log decryption request: %w", err)
		}

		// Decrypt fields
		log, err = s.decryptLog(log)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt log: %w", err)
		}
	}

	return log, nil
}

// GetLogsByRequestID retrieves logs by request ID
func (s *LogService) GetLogsByRequestID(ctx context.Context, userID int, requestID string) ([]entities.ClickHouseLogRecord, error) {
	// Check search permission
	hasPermission, err := s.authChecker.HasPermission(ctx, userID, "logs:search")
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}
	if !hasPermission {
		return nil, fmt.Errorf("permission denied: logs:search required")
	}

	logs, err := s.logReader.GetLogsByRequestID(ctx, requestID)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs by request ID: %w", err)
	}

	return logs, nil
}

// QueryLogs retrieves logs based on filter criteria
func (s *LogService) QueryLogs(ctx context.Context, userID int, filter LogFilter) ([]entities.ClickHouseLogRecord, error) {
	// Check filter permission
	hasPermission, err := s.authChecker.HasPermission(ctx, userID, "logs:filter")
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}
	if !hasPermission {
		return nil, fmt.Errorf("permission denied: logs:filter required")
	}

	logs, err := s.logReader.QueryLogs(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}

	return logs, nil
}

// GetLogsStats retrieves statistics about logs
func (s *LogService) GetLogsStats(ctx context.Context, userID int, filter LogFilter) (map[string]interface{}, error) {
	// Check stats permission
	hasPermission, err := s.authChecker.HasPermission(ctx, userID, "logs:stats")
	if err != nil {
		return nil, fmt.Errorf("failed to check permission: %w", err)
	}
	if !hasPermission {
		return nil, fmt.Errorf("permission denied: logs:stats required")
	}

	stats, err := s.logReader.GetLogsStats(ctx, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs stats: %w", err)
	}

	return stats, nil
}

// ExportLogsToCSV exports logs to CSV format
func (s *LogService) ExportLogsToCSV(ctx context.Context, userID int, filter LogFilter, writer io.Writer) error {
	// Check filter permission
	hasPermission, err := s.authChecker.HasPermission(ctx, userID, "logs:filter")
	if err != nil {
		return fmt.Errorf("failed to check permission: %w", err)
	}
	if !hasPermission {
		return fmt.Errorf("permission denied: logs:filter required")
	}

	logs, err := s.logReader.QueryLogs(ctx, filter)
	if err != nil {
		return fmt.Errorf("failed to query logs: %w", err)
	}

	csvWriter := csv.NewWriter(writer)
	defer csvWriter.Flush()

	// Write header
	header := []string{
		"log_id", "timestamp", "integration_id", "request_id",
		"http_method", "endpoint", "request_body", "response_body",
		"duration_ms", "status_code", "success", "error_message", "user_id",
	}
	if err := csvWriter.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	// Write rows
	for _, log := range logs {
		row := []string{
			log.LogID,
			log.Timestamp.Format(time.RFC3339),
			log.IntegrationID,
			log.RequestID,
			log.HTTPMethod,
			log.Endpoint,
			log.RequestBody,
			log.ResponseBody,
			fmt.Sprintf("%d", log.DurationMs),
			fmt.Sprintf("%d", log.StatusCode),
			fmt.Sprintf("%t", log.Success),
			log.ErrorMessage,
			fmt.Sprintf("%v", log.UserID),
		}
		if err := csvWriter.Write(row); err != nil {
			return fmt.Errorf("failed to write CSV row: %w", err)
		}
	}

	return nil
}

// decryptLog decrypts sensitive fields in a log record
func (s *LogService) decryptLog(log *entities.ClickHouseLogRecord) (*entities.ClickHouseLogRecord, error) {
	// Decrypt request body
	decryptedRequestBody, err := s.encrypter.DecryptFields([]byte(log.RequestBody), s.sensitiveFields)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt request body: %w", err)
	}
	log.RequestBody = string(decryptedRequestBody)

	// Decrypt response body
	decryptedResponseBody, err := s.encrypter.DecryptFields([]byte(log.ResponseBody), s.sensitiveFields)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt response body: %w", err)
	}
	log.ResponseBody = string(decryptedResponseBody)

	return log, nil
}
