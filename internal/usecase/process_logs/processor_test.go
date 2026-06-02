package process_logs

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Baldyozh/log-processor/internal/domain/entities"
)

type fakeProcessEncrypter struct {
	fail         bool
	failOnSecond bool
	calls        int32
}

func (f *fakeProcessEncrypter) EncryptFields(data []byte, fields []string) ([]byte, error) {
	call := atomic.AddInt32(&f.calls, 1)
	if f.fail {
		return nil, errors.New("encryption failed")
	}
	if f.failOnSecond && call == 2 {
		return nil, errors.New("response encryption failed")
	}
	return append([]byte("encrypted:"), data...), nil
}

type fakeProcessStorage struct {
	mu      sync.Mutex
	batches [][]entities.ClickHouseLogRecord
	err     error
}

func (f *fakeProcessStorage) InsertBatch(records []entities.ClickHouseLogRecord) error {
	if f.err != nil {
		return f.err
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	copied := append([]entities.ClickHouseLogRecord(nil), records...)
	f.batches = append(f.batches, copied)
	return nil
}

func (f *fakeProcessStorage) batchCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.batches)
}

func TestEncryptWorkerEncryptsRequestAndResponseBodies(t *testing.T) {
	processor := NewLogProcessor(
		nil,
		&fakeProcessEncrypter{},
		nil,
		[]string{"passport"},
		WorkerPoolConfig{WorkerCount: 1, ChannelBufferSize: 1},
		BatchConfig{Size: 10, FlushInterval: time.Hour},
	)

	inCh := make(chan entities.Log, 1)
	outCh := make(chan entities.ClickHouseLogRecord, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	go processor.encryptWorker(1, inCh, outCh, &wg)
	inCh <- entities.Log{
		LogBody: entities.ClickHouseLogRecord{
			LogID:        "log-1",
			RequestBody:  `{"passport":"1234"}`,
			ResponseBody: `{"snils":"5678"}`,
		},
	}
	close(inCh)
	wg.Wait()

	select {
	case got := <-outCh:
		if got.RequestBody != `encrypted:{"passport":"1234"}` {
			t.Fatalf("RequestBody = %q", got.RequestBody)
		}
		if got.ResponseBody != `encrypted:{"snils":"5678"}` {
			t.Fatalf("ResponseBody = %q", got.ResponseBody)
		}
	default:
		t.Fatal("expected encrypted log record")
	}
}

func TestEncryptWorkerSkipsRecordWhenEncryptionFails(t *testing.T) {
	processor := NewLogProcessor(
		nil,
		&fakeProcessEncrypter{fail: true},
		nil,
		[]string{"passport"},
		WorkerPoolConfig{WorkerCount: 1, ChannelBufferSize: 1},
		BatchConfig{Size: 10, FlushInterval: time.Hour},
	)

	inCh := make(chan entities.Log, 1)
	outCh := make(chan entities.ClickHouseLogRecord, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	go processor.encryptWorker(1, inCh, outCh, &wg)
	inCh <- entities.Log{LogBody: entities.ClickHouseLogRecord{LogID: "log-1"}}
	close(inCh)
	wg.Wait()

	select {
	case got := <-outCh:
		t.Fatalf("expected no output record, got %#v", got)
	default:
	}
}

func TestEncryptWorkerSkipsRecordWhenResponseEncryptionFails(t *testing.T) {
	processor := NewLogProcessor(
		nil,
		&fakeProcessEncrypter{failOnSecond: true},
		nil,
		[]string{"passport"},
		WorkerPoolConfig{WorkerCount: 1, ChannelBufferSize: 1},
		BatchConfig{Size: 10, FlushInterval: time.Hour},
	)

	inCh := make(chan entities.Log, 1)
	outCh := make(chan entities.ClickHouseLogRecord, 1)
	var wg sync.WaitGroup
	wg.Add(1)

	go processor.encryptWorker(1, inCh, outCh, &wg)
	inCh <- entities.Log{LogBody: entities.ClickHouseLogRecord{
		LogID:        "log-1",
		RequestBody:  `{"passport":"1234"}`,
		ResponseBody: `{"snils":"5678"}`,
	}}
	close(inCh)
	wg.Wait()

	select {
	case got := <-outCh:
		t.Fatalf("expected no output record, got %#v", got)
	default:
	}
}

func TestBatchWriterFlushesWhenBatchSizeReached(t *testing.T) {
	storage := &fakeProcessStorage{}
	processor := NewLogProcessor(
		nil,
		&fakeProcessEncrypter{},
		storage,
		nil,
		WorkerPoolConfig{},
		BatchConfig{Size: 2, FlushInterval: time.Hour},
	)

	inCh := make(chan entities.ClickHouseLogRecord, 2)
	doneCh := make(chan struct{})
	writerDone := make(chan struct{})

	go processor.batchWriter(inCh, doneCh, writerDone)
	inCh <- entities.ClickHouseLogRecord{LogID: "log-1"}
	inCh <- entities.ClickHouseLogRecord{LogID: "log-2"}

	waitForBatchCount(t, storage, 1)

	close(doneCh)
	<-writerDone
}

func TestBatchWriterFlushesRemainingRecordsOnDone(t *testing.T) {
	storage := &fakeProcessStorage{}
	processor := NewLogProcessor(
		nil,
		&fakeProcessEncrypter{},
		storage,
		nil,
		WorkerPoolConfig{},
		BatchConfig{Size: 10, FlushInterval: time.Hour},
	)

	inCh := make(chan entities.ClickHouseLogRecord)
	doneCh := make(chan struct{})
	writerDone := make(chan struct{})

	go processor.batchWriter(inCh, doneCh, writerDone)
	inCh <- entities.ClickHouseLogRecord{LogID: "log-1"}
	close(doneCh)
	<-writerDone

	if storage.batchCount() != 1 {
		t.Fatalf("batch count = %d, want 1", storage.batchCount())
	}
}

func TestBatchWriterFlushesWhenInputChannelCloses(t *testing.T) {
	storage := &fakeProcessStorage{}
	processor := NewLogProcessor(
		nil,
		&fakeProcessEncrypter{},
		storage,
		nil,
		WorkerPoolConfig{},
		BatchConfig{Size: 10, FlushInterval: time.Hour},
	)

	inCh := make(chan entities.ClickHouseLogRecord, 1)
	doneCh := make(chan struct{})
	writerDone := make(chan struct{})

	go processor.batchWriter(inCh, doneCh, writerDone)
	inCh <- entities.ClickHouseLogRecord{LogID: "log-1"}
	close(inCh)
	<-writerDone

	if storage.batchCount() != 1 {
		t.Fatalf("batch count = %d, want 1", storage.batchCount())
	}
}

func TestBatchWriterFlushesOnTicker(t *testing.T) {
	storage := &fakeProcessStorage{}
	processor := NewLogProcessor(
		nil,
		&fakeProcessEncrypter{},
		storage,
		nil,
		WorkerPoolConfig{},
		BatchConfig{Size: 10, FlushInterval: 5 * time.Millisecond},
	)

	inCh := make(chan entities.ClickHouseLogRecord, 1)
	doneCh := make(chan struct{})
	writerDone := make(chan struct{})

	go processor.batchWriter(inCh, doneCh, writerDone)
	inCh <- entities.ClickHouseLogRecord{LogID: "log-1"}

	waitForBatchCount(t, storage, 1)
	close(doneCh)
	<-writerDone
}

func TestFlushBatchHandlesStorageError(t *testing.T) {
	storage := &fakeProcessStorage{err: errors.New("insert failed")}
	processor := NewLogProcessor(
		nil,
		&fakeProcessEncrypter{},
		storage,
		nil,
		WorkerPoolConfig{},
		BatchConfig{Size: 10, FlushInterval: time.Hour},
	)

	processor.flushBatch([]entities.ClickHouseLogRecord{{LogID: "log-1"}})

	if storage.batchCount() != 0 {
		t.Fatalf("batch count = %d, want 0", storage.batchCount())
	}
}

type fakeProcessConsumer struct {
	logs         []entities.Log
	err          error
	readCalls    int32
	closeDone    chan struct{}
	onSecondRead func()
}

func (f *fakeProcessConsumer) ReadLog() (entities.Log, error) {
	call := atomic.AddInt32(&f.readCalls, 1)
	if f.closeDone != nil && call == 1 {
		close(f.closeDone)
	}
	if f.onSecondRead != nil && call == 2 {
		f.onSecondRead()
	}
	if f.err != nil {
		return entities.Log{}, f.err
	}
	if len(f.logs) == 0 {
		return entities.Log{}, errors.New("no logs")
	}
	logEntry := f.logs[0]
	f.logs = f.logs[1:]
	return logEntry, nil
}

func (f *fakeProcessConsumer) Close() error {
	return nil
}

func TestProcessLogsStopsWhenDoneIsClosed(t *testing.T) {
	doneCh := make(chan struct{})
	close(doneCh)
	processor := NewLogProcessor(
		&fakeProcessConsumer{},
		&fakeProcessEncrypter{},
		&fakeProcessStorage{},
		nil,
		WorkerPoolConfig{WorkerCount: 1, ChannelBufferSize: 1},
		BatchConfig{Size: 10, FlushInterval: time.Hour},
	)

	processor.ProcessLogs(doneCh)
}

func TestProcessLogsReadsEncryptsAndStoresLog(t *testing.T) {
	doneCh := make(chan struct{})
	storage := &fakeProcessStorage{}
	consumer := &fakeProcessConsumer{
		logs: []entities.Log{{
			LogBody: entities.ClickHouseLogRecord{
				LogID:        "log-1",
				RequestBody:  `{"passport":"1234"}`,
				ResponseBody: `{"snils":"5678"}`,
			},
		}},
	}
	consumer.onSecondRead = func() {
		waitForBatchCount(t, storage, 1)
		close(doneCh)
	}
	processor := NewLogProcessor(
		consumer,
		&fakeProcessEncrypter{},
		storage,
		[]string{"passport", "snils"},
		WorkerPoolConfig{WorkerCount: 1, ChannelBufferSize: 2},
		BatchConfig{Size: 1, FlushInterval: time.Hour},
	)

	processor.ProcessLogs(doneCh)

	waitForBatchCount(t, storage, 1)
}

func waitForBatchCount(t *testing.T, storage *fakeProcessStorage, want int) {
	t.Helper()

	deadline := time.After(time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-deadline:
			t.Fatalf("batch count = %d, want %d", storage.batchCount(), want)
		case <-ticker.C:
			if storage.batchCount() == want {
				return
			}
		}
	}
}
