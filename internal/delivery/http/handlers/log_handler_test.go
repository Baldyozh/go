package handlers

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestParseLogFilter_AllSupportedParameters(t *testing.T) {
	req := httptest.NewRequest(
		"GET",
		"/api/logs?request_id=req-1&integration_id=partner-1&start_time=2026-05-17T10:00:00Z&end_time=2026-05-17T11:00:00Z&status_code=500&http_method=POST&endpoint=/api/v1/users&user_id=42&success=false",
		nil,
	)

	filter, err := parseLogFilter(req)
	if err != nil {
		t.Fatalf("parseLogFilter returned error: %v", err)
	}

	if filter.RequestID != "req-1" {
		t.Fatalf("RequestID = %q, want req-1", filter.RequestID)
	}
	if filter.IntegrationID != "partner-1" {
		t.Fatalf("IntegrationID = %q, want partner-1", filter.IntegrationID)
	}
	if filter.HTTPMethod != "POST" {
		t.Fatalf("HTTPMethod = %q, want POST", filter.HTTPMethod)
	}
	if filter.Endpoint != "/api/v1/users" {
		t.Fatalf("Endpoint = %q, want /api/v1/users", filter.Endpoint)
	}
	if filter.StatusCode == nil || *filter.StatusCode != 500 {
		t.Fatalf("StatusCode = %v, want 500", filter.StatusCode)
	}
	if filter.UserID == nil || *filter.UserID != 42 {
		t.Fatalf("UserID = %v, want 42", filter.UserID)
	}
	if filter.Success == nil || *filter.Success != false {
		t.Fatalf("Success = %v, want false", filter.Success)
	}

	wantStart := time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC)
	if filter.StartTime == nil || !filter.StartTime.Equal(wantStart) {
		t.Fatalf("StartTime = %v, want %v", filter.StartTime, wantStart)
	}
}

func TestParseLogFilter_InvalidValuesReturnError(t *testing.T) {
	tests := []struct {
		name string
		url  string
	}{
		{
			name: "invalid start time",
			url:  "/api/logs?start_time=17-05-2026",
		},
		{
			name: "invalid status code",
			url:  "/api/logs?status_code=abc",
		},
		{
			name: "invalid user id",
			url:  "/api/logs?user_id=user",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.url, nil)

			if _, err := parseLogFilter(req); err == nil {
				t.Fatal("expected parse error, got nil")
			}
		})
	}
}
