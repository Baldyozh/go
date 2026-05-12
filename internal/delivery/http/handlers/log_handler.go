package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/Baldyozh/log-processor/internal/infrastructure/auth"
	"github.com/Baldyozh/log-processor/internal/usecase/manage_logs"
	"github.com/go-chi/chi/v5"
)

// LogHandler handles HTTP requests for log operations
type LogHandler struct {
	logService *manage_logs.LogService
}

// NewLogHandler creates a new log handler
func NewLogHandler(logService *manage_logs.LogService) *LogHandler {
	return &LogHandler{logService: logService}
}

// GetLogByID retrieves a log by ID
func (h *LogHandler) GetLogByID(w http.ResponseWriter, r *http.Request) {
	logID := chi.URLParam(r, "id")
	if logID == "" {
		http.Error(w, "log_id is required", http.StatusBadRequest)
		return
	}

	userID, ok := auth.GetUserIDFromContext(r)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	// Check if decryption is requested
	decrypt := r.URL.Query().Get("decrypt") == "true"
	reason := r.URL.Query().Get("reason")

	log, err := h.logService.GetLogByID(r.Context(), userID, logID, decrypt, reason)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, log)
}

// GetLogsByRequestID retrieves logs by request ID
func (h *LogHandler) GetLogsByRequestID(w http.ResponseWriter, r *http.Request) {
	requestID := chi.URLParam(r, "request_id")
	if requestID == "" {
		http.Error(w, "request_id is required", http.StatusBadRequest)
		return
	}

	userID, ok := auth.GetUserIDFromContext(r)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	logs, err := h.logService.GetLogsByRequestID(r.Context(), userID, requestID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, logs)
}

// QueryLogs retrieves logs based on filter criteria
func (h *LogHandler) QueryLogs(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserIDFromContext(r)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	filter, err := parseLogFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	logs, err := h.logService.QueryLogs(r.Context(), userID, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, logs)
}

// GetLogsStats retrieves statistics about logs
func (h *LogHandler) GetLogsStats(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserIDFromContext(r)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	filter, err := parseLogFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	stats, err := h.logService.GetLogsStats(r.Context(), userID, filter)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, http.StatusOK, stats)
}

// ExportLogsToCSV exports logs to CSV format
func (h *LogHandler) ExportLogsToCSV(w http.ResponseWriter, r *http.Request) {
	userID, ok := auth.GetUserIDFromContext(r)
	if !ok {
		http.Error(w, "User not authenticated", http.StatusUnauthorized)
		return
	}

	filter, err := parseLogFilter(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", "attachment; filename=logs.csv")

	if err := h.logService.ExportLogsToCSV(r.Context(), userID, filter, w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

// parseLogFilter parses filter parameters from request
func parseLogFilter(r *http.Request) (manage_logs.LogFilter, error) {
	query := r.URL.Query()
	filter := manage_logs.LogFilter{}

	filter.RequestID = query.Get("request_id")
	filter.IntegrationID = query.Get("integration_id")
	filter.HTTPMethod = query.Get("http_method")
	filter.Endpoint = query.Get("endpoint")

	if startTimeStr := query.Get("start_time"); startTimeStr != "" {
		startTime, err := time.Parse(time.RFC3339, startTimeStr)
		if err != nil {
			return filter, err
		}
		filter.StartTime = &startTime
	}

	if endTimeStr := query.Get("end_time"); endTimeStr != "" {
		endTime, err := time.Parse(time.RFC3339, endTimeStr)
		if err != nil {
			return filter, err
		}
		filter.EndTime = &endTime
	}

	if statusCodeStr := query.Get("status_code"); statusCodeStr != "" {
		statusCode, err := strconv.ParseUint(statusCodeStr, 10, 16)
		if err != nil {
			return filter, err
		}
		sc := uint16(statusCode)
		filter.StatusCode = &sc
	}

	if userIDStr := query.Get("user_id"); userIDStr != "" {
		userID, err := strconv.ParseUint(userIDStr, 10, 32)
		if err != nil {
			return filter, err
		}
		uid := uint32(userID)
		filter.UserID = &uid
	}

	if successStr := query.Get("success"); successStr != "" {
		success := successStr == "true"
		filter.Success = &success
	}

	return filter, nil
}

// respondJSON writes JSON response
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
