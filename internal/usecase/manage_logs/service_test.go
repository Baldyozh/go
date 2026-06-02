package manage_logs

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
)

type fakeLogReader struct {
	log          *entities.ClickHouseLogRecord
	logs         []entities.ClickHouseLogRecord
	stats        map[string]interface{}
	lastFilter   LogFilter
	getByIDCalls int
	queryCalls   int
	getByIDErr   error
	requestErr   error
	queryErr     error
	statsErr     error
}

func (f *fakeLogReader) GetLogByID(ctx context.Context, logID string) (*entities.ClickHouseLogRecord, error) {
	f.getByIDCalls++
	if f.getByIDErr != nil {
		return nil, f.getByIDErr
	}
	if f.log == nil {
		return nil, errors.New("not found")
	}
	return f.log, nil
}

func (f *fakeLogReader) GetLogsByRequestID(ctx context.Context, requestID string) ([]entities.ClickHouseLogRecord, error) {
	if f.requestErr != nil {
		return nil, f.requestErr
	}
	return f.logs, nil
}

func (f *fakeLogReader) QueryLogs(ctx context.Context, filter LogFilter) ([]entities.ClickHouseLogRecord, error) {
	f.queryCalls++
	f.lastFilter = filter
	if f.queryErr != nil {
		return nil, f.queryErr
	}
	return f.logs, nil
}

func (f *fakeLogReader) GetLogsStats(ctx context.Context, filter LogFilter) (map[string]interface{}, error) {
	f.lastFilter = filter
	if f.statsErr != nil {
		return nil, f.statsErr
	}
	return f.stats, nil
}

type fakeManageEncrypter struct {
	calls int
	err   error
}

func (f *fakeManageEncrypter) DecryptFields(data []byte, fields []string) ([]byte, error) {
	f.calls++
	if f.err != nil {
		return nil, f.err
	}
	return []byte(strings.ReplaceAll(string(data), "encrypted", "decrypted")), nil
}

type fakeAuthChecker struct {
	permissions      map[string]bool
	checked          []string
	decryptionLogID  string
	decryptionReason string
	permissionErr    error
	auditErr         error
}

func (f *fakeAuthChecker) HasPermission(ctx context.Context, userID int, permissionName string) (bool, error) {
	f.checked = append(f.checked, permissionName)
	if f.permissionErr != nil {
		return false, f.permissionErr
	}
	return f.permissions[permissionName], nil
}

func (f *fakeAuthChecker) LogDecryptionRequest(ctx context.Context, userID int, logID string, reason string) error {
	if f.auditErr != nil {
		return f.auditErr
	}
	f.decryptionLogID = logID
	f.decryptionReason = reason
	return nil
}

func TestLogService_GetLogByIDRequiresReadPermission(t *testing.T) {
	reader := &fakeLogReader{}
	service := NewLogService(reader, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:read": false},
	}, []string{"passport"})

	_, err := service.GetLogByID(context.Background(), 7, "log-1", false, "")
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}
	if reader.getByIDCalls != 0 {
		t.Fatalf("reader called %d times, want 0", reader.getByIDCalls)
	}
}

func TestLogService_GetLogByIDDecryptsAndAuditsWhenAllowed(t *testing.T) {
	reader := &fakeLogReader{
		log: &entities.ClickHouseLogRecord{
			LogID:        "log-1",
			RequestBody:  `{"passport":"encrypted-request"}`,
			ResponseBody: `{"snils":"encrypted-response"}`,
		},
	}
	encrypter := &fakeManageEncrypter{}
	auth := &fakeAuthChecker{
		permissions: map[string]bool{
			"logs:read":    true,
			"logs:decrypt": true,
		},
	}
	service := NewLogService(reader, encrypter, auth, []string{"passport", "snils"})

	got, err := service.GetLogByID(context.Background(), 7, "log-1", true, "incident check")
	if err != nil {
		t.Fatalf("GetLogByID returned error: %v", err)
	}

	if !strings.Contains(got.RequestBody, "decrypted-request") {
		t.Fatalf("request body was not decrypted: %s", got.RequestBody)
	}
	if !strings.Contains(got.ResponseBody, "decrypted-response") {
		t.Fatalf("response body was not decrypted: %s", got.ResponseBody)
	}
	if encrypter.calls != 2 {
		t.Fatalf("DecryptFields called %d times, want 2", encrypter.calls)
	}
	if auth.decryptionLogID != "log-1" || auth.decryptionReason != "incident check" {
		t.Fatalf("decryption audit mismatch: id=%q reason=%q", auth.decryptionLogID, auth.decryptionReason)
	}
}

func TestLogService_GetLogByIDRejectsDecryptWithoutPermission(t *testing.T) {
	reader := &fakeLogReader{
		log: &entities.ClickHouseLogRecord{LogID: "log-1"},
	}
	auth := &fakeAuthChecker{
		permissions: map[string]bool{
			"logs:read":    true,
			"logs:decrypt": false,
		},
	}
	service := NewLogService(reader, &fakeManageEncrypter{}, auth, []string{"passport"})

	_, err := service.GetLogByID(context.Background(), 7, "log-1", true, "incident check")
	if err == nil {
		t.Fatal("expected decrypt permission error, got nil")
	}
	if auth.decryptionLogID != "" {
		t.Fatalf("decryption request should not be audited when permission is denied")
	}
}

func TestLogService_QueryLogsRequiresFilterPermission(t *testing.T) {
	reader := &fakeLogReader{}
	service := NewLogService(reader, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:filter": false},
	}, nil)

	_, err := service.QueryLogs(context.Background(), 7, LogFilter{RequestID: "req-1"})
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}
	if reader.queryCalls != 0 {
		t.Fatalf("QueryLogs called %d times, want 0", reader.queryCalls)
	}
}

func TestLogService_ExportLogsToCSV(t *testing.T) {
	userID := uint32(42)
	ts := time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC)
	reader := &fakeLogReader{
		logs: []entities.ClickHouseLogRecord{
			{
				LogID:         "log-1",
				Timestamp:     ts,
				IntegrationID: "partner-1",
				RequestID:     "req-1",
				HTTPMethod:    "POST",
				Endpoint:      "/api/v1/users",
				RequestBody:   `{"passport":"hidden"}`,
				ResponseBody:  `{"ok":true}`,
				DurationMs:    150,
				StatusCode:    200,
				Success:       true,
				UserID:        &userID,
			},
		},
	}
	service := NewLogService(reader, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:filter": true},
	}, nil)

	var buf bytes.Buffer
	err := service.ExportLogsToCSV(context.Background(), 7, LogFilter{IntegrationID: "partner-1"}, &buf)
	if err != nil {
		t.Fatalf("ExportLogsToCSV returned error: %v", err)
	}

	csv := buf.String()
	if !strings.Contains(csv, "log_id,timestamp,integration_id") {
		t.Fatalf("CSV header missing: %s", csv)
	}
	if !strings.Contains(csv, "log-1,2026-05-17T10:00:00Z,partner-1,req-1") {
		t.Fatalf("CSV row missing: %s", csv)
	}
	if reader.lastFilter.IntegrationID != "partner-1" {
		t.Fatalf("filter was not forwarded to reader: %#v", reader.lastFilter)
	}
}

func TestLogService_GetLogByIDPermissionCheckError(t *testing.T) {
	service := NewLogService(&fakeLogReader{}, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissionErr: errors.New("postgres unavailable"),
	}, nil)

	_, err := service.GetLogByID(context.Background(), 7, "log-1", false, "")
	if err == nil {
		t.Fatal("expected permission check error, got nil")
	}
}

func TestLogService_GetLogByIDReaderError(t *testing.T) {
	service := NewLogService(&fakeLogReader{getByIDErr: errors.New("clickhouse unavailable")}, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:read": true},
	}, nil)

	_, err := service.GetLogByID(context.Background(), 7, "log-1", false, "")
	if err == nil {
		t.Fatal("expected reader error, got nil")
	}
}

func TestLogService_GetLogByIDDecryptAuditError(t *testing.T) {
	service := NewLogService(&fakeLogReader{
		log: &entities.ClickHouseLogRecord{LogID: "log-1"},
	}, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:read": true, "logs:decrypt": true},
		auditErr:    errors.New("audit failed"),
	}, nil)

	_, err := service.GetLogByID(context.Background(), 7, "log-1", true, "incident")
	if err == nil {
		t.Fatal("expected audit error, got nil")
	}
}

func TestLogService_GetLogByIDDecryptError(t *testing.T) {
	service := NewLogService(&fakeLogReader{
		log: &entities.ClickHouseLogRecord{
			LogID:        "log-1",
			RequestBody:  `{"passport":"encrypted"}`,
			ResponseBody: `{"ok":true}`,
		},
	}, &fakeManageEncrypter{err: errors.New("decrypt failed")}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:read": true, "logs:decrypt": true},
	}, []string{"passport"})

	_, err := service.GetLogByID(context.Background(), 7, "log-1", true, "incident")
	if err == nil {
		t.Fatal("expected decrypt error, got nil")
	}
}

func TestLogService_GetLogsByRequestIDSuccess(t *testing.T) {
	reader := &fakeLogReader{logs: []entities.ClickHouseLogRecord{{LogID: "log-1", RequestID: "req-1"}}}
	service := NewLogService(reader, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:search": true},
	}, nil)

	logs, err := service.GetLogsByRequestID(context.Background(), 7, "req-1")
	if err != nil {
		t.Fatalf("GetLogsByRequestID returned error: %v", err)
	}
	if len(logs) != 1 || logs[0].LogID != "log-1" {
		t.Fatalf("logs = %#v", logs)
	}
}

func TestLogService_GetLogsByRequestIDPermissionDenied(t *testing.T) {
	service := NewLogService(&fakeLogReader{}, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:search": false},
	}, nil)

	_, err := service.GetLogsByRequestID(context.Background(), 7, "req-1")
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}
}

func TestLogService_GetLogsByRequestIDReaderError(t *testing.T) {
	service := NewLogService(&fakeLogReader{requestErr: errors.New("read failed")}, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:search": true},
	}, nil)

	_, err := service.GetLogsByRequestID(context.Background(), 7, "req-1")
	if err == nil {
		t.Fatal("expected reader error, got nil")
	}
}

func TestLogService_QueryLogsSuccess(t *testing.T) {
	reader := &fakeLogReader{logs: []entities.ClickHouseLogRecord{{LogID: "log-1"}}}
	service := NewLogService(reader, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:filter": true},
	}, nil)

	logs, err := service.QueryLogs(context.Background(), 7, LogFilter{IntegrationID: "partner-1"})
	if err != nil {
		t.Fatalf("QueryLogs returned error: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("logs length = %d, want 1", len(logs))
	}
	if reader.lastFilter.IntegrationID != "partner-1" {
		t.Fatalf("filter mismatch: %#v", reader.lastFilter)
	}
}

func TestLogService_QueryLogsReaderError(t *testing.T) {
	service := NewLogService(&fakeLogReader{queryErr: errors.New("query failed")}, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:filter": true},
	}, nil)

	_, err := service.QueryLogs(context.Background(), 7, LogFilter{})
	if err == nil {
		t.Fatal("expected query error, got nil")
	}
}

func TestLogService_GetLogsStatsSuccess(t *testing.T) {
	reader := &fakeLogReader{stats: map[string]interface{}{"total_logs": 10}}
	service := NewLogService(reader, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:stats": true},
	}, nil)

	stats, err := service.GetLogsStats(context.Background(), 7, LogFilter{RequestID: "req-1"})
	if err != nil {
		t.Fatalf("GetLogsStats returned error: %v", err)
	}
	if stats["total_logs"] != 10 {
		t.Fatalf("stats = %#v", stats)
	}
	if reader.lastFilter.RequestID != "req-1" {
		t.Fatalf("filter mismatch: %#v", reader.lastFilter)
	}
}

func TestLogService_GetLogsStatsPermissionDenied(t *testing.T) {
	service := NewLogService(&fakeLogReader{}, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:stats": false},
	}, nil)

	_, err := service.GetLogsStats(context.Background(), 7, LogFilter{})
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}
}

func TestLogService_GetLogsStatsReaderError(t *testing.T) {
	service := NewLogService(&fakeLogReader{statsErr: errors.New("stats failed")}, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:stats": true},
	}, nil)

	_, err := service.GetLogsStats(context.Background(), 7, LogFilter{})
	if err == nil {
		t.Fatal("expected stats error, got nil")
	}
}

func TestLogService_ExportLogsToCSVPermissionDenied(t *testing.T) {
	service := NewLogService(&fakeLogReader{}, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:filter": false},
	}, nil)

	var buf bytes.Buffer
	err := service.ExportLogsToCSV(context.Background(), 7, LogFilter{}, &buf)
	if err == nil {
		t.Fatal("expected permission error, got nil")
	}
}

func TestLogService_ExportLogsToCSVQueryError(t *testing.T) {
	service := NewLogService(&fakeLogReader{queryErr: errors.New("query failed")}, &fakeManageEncrypter{}, &fakeAuthChecker{
		permissions: map[string]bool{"logs:filter": true},
	}, nil)

	var buf bytes.Buffer
	err := service.ExportLogsToCSV(context.Background(), 7, LogFilter{}, &buf)
	if err == nil {
		t.Fatal("expected query error, got nil")
	}
}
