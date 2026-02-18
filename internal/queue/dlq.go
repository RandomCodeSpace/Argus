package queue

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// DeadLetterQueue provides disk-based resilience for failed database writes.
// When a batch insert fails, the data is serialized to JSON and written to disk.
// A background replay worker periodically attempts to re-insert failed batches.
type DeadLetterQueue struct {
	dir      string
	interval time.Duration
	replayFn func(data []byte) error
	stopCh   chan struct{}
	wg       sync.WaitGroup
	mu       sync.Mutex
}

// NewDLQ creates a new Dead Letter Queue that stores failed batches in the given directory.
// replayFn is called with raw JSON bytes during replay — the caller provides the deserialization + insert logic.
func NewDLQ(dir string, interval time.Duration, replayFn func(data []byte) error) (*DeadLetterQueue, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create DLQ directory %s: %w", dir, err)
	}

	dlq := &DeadLetterQueue{
		dir:      dir,
		interval: interval,
		replayFn: replayFn,
		stopCh:   make(chan struct{}),
	}

	dlq.wg.Add(1)
	go dlq.replayWorker()

	slog.Info("🔁 DLQ replay worker started", "dir", dir, "interval", interval)
	return dlq, nil
}

// Enqueue serializes the given batch to JSON and writes it to disk.
// This is called when a database batch insert fails.
func (d *DeadLetterQueue) Enqueue(batch interface{}) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	data, err := json.Marshal(batch)
	if err != nil {
		return fmt.Errorf("DLQ: failed to marshal batch: %w", err)
	}

	filename := fmt.Sprintf("batch_%d.json", time.Now().UnixNano())
	path := filepath.Join(d.dir, filename)

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("DLQ: failed to write file %s: %w", path, err)
	}

	slog.Warn("📦 Batch written to DLQ", "file", filename, "bytes", len(data))
	return nil
}

// Size returns the number of files currently in the DLQ directory.
func (d *DeadLetterQueue) Size() int {
	d.mu.Lock()
	defer d.mu.Unlock()

	entries, err := os.ReadDir(d.dir)
	if err != nil {
		slog.Error("DLQ: failed to read directory", "error", err)
		return 0
	}

	count := 0
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" {
			count++
		}
	}
	return count
}

// Stop gracefully shuts down the replay worker.
func (d *DeadLetterQueue) Stop() {
	close(d.stopCh)
	d.wg.Wait()
	slog.Info("🛑 DLQ replay worker stopped")
}

// replayWorker periodically scans the DLQ directory and attempts to re-insert failed batches.
func (d *DeadLetterQueue) replayWorker() {
	defer d.wg.Done()

	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()

	for {
		select {
		case <-d.stopCh:
			return
		case <-ticker.C:
			d.processFiles()
		}
	}
}

// processFiles reads all JSON files in the DLQ directory and attempts to replay them.
func (d *DeadLetterQueue) processFiles() {
	d.mu.Lock()
	entries, err := os.ReadDir(d.dir)
	d.mu.Unlock()

	if err != nil {
		slog.Error("DLQ: failed to read directory for replay", "error", err)
		return
	}

	replayed := 0
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		path := filepath.Join(d.dir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			slog.Error("DLQ: failed to read file", "file", entry.Name(), "error", err)
			continue
		}

		if err := d.replayFn(data); err != nil {
			slog.Warn("DLQ: replay failed, will retry later", "file", entry.Name(), "error", err)
			continue
		}

		// Success — remove the file
		d.mu.Lock()
		if err := os.Remove(path); err != nil {
			slog.Error("DLQ: failed to remove replayed file", "file", entry.Name(), "error", err)
		} else {
			replayed++
			slog.Info("✅ DLQ file replayed and removed", "file", entry.Name())
		}
		d.mu.Unlock()
	}

	if replayed > 0 {
		slog.Info("🔁 DLQ replay cycle complete", "replayed", replayed)
	}
}
