package add_logs

import (
	"context"
	"errors"
	"testing"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
)

type fakeLogProducer struct {
	written []entities.Log
	err     error
}

func (f *fakeLogProducer) WriteLog(ctx context.Context, log entities.Log) error {
	if f.err != nil {
		return f.err
	}
	f.written = append(f.written, log)
	return nil
}

func (f *fakeLogProducer) Close() error {
	return nil
}

func TestLogAdder_AddLogsWritesEveryLog(t *testing.T) {
	producer := &fakeLogProducer{}
	adder := NewLogAdder(producer)
	logs := []entities.Log{
		{LogBody: entities.ClickHouseLogRecord{LogID: "log-1"}},
		{LogBody: entities.ClickHouseLogRecord{LogID: "log-2"}},
	}

	if err := adder.AddLogs(context.Background(), logs); err != nil {
		t.Fatalf("AddLogs returned error: %v", err)
	}

	if len(producer.written) != len(logs) {
		t.Fatalf("written logs = %d, want %d", len(producer.written), len(logs))
	}
	if producer.written[1].LogBody.LogID != "log-2" {
		t.Fatalf("second log id = %q, want log-2", producer.written[1].LogBody.LogID)
	}
}

func TestLogAdder_AddLogsStopsOnProducerError(t *testing.T) {
	wantErr := errors.New("kafka unavailable")
	producer := &fakeLogProducer{err: wantErr}
	adder := NewLogAdder(producer)

	err := adder.AddLogs(context.Background(), []entities.Log{
		{LogBody: entities.ClickHouseLogRecord{LogID: "log-1"}},
		{LogBody: entities.ClickHouseLogRecord{LogID: "log-2"}},
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("AddLogs error = %v, want %v", err, wantErr)
	}
	if len(producer.written) != 0 {
		t.Fatalf("written logs = %d, want 0", len(producer.written))
	}
}

func TestLogAdder_AddLogsEmptySlice(t *testing.T) {
	producer := &fakeLogProducer{}
	adder := NewLogAdder(producer)

	if err := adder.AddLogs(context.Background(), nil); err != nil {
		t.Fatalf("AddLogs returned error: %v", err)
	}
	if len(producer.written) != 0 {
		t.Fatalf("written logs = %d, want 0", len(producer.written))
	}
}
