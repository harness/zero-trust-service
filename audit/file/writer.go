package file

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit"
)

// Writer persists audit records to local files.
// Layout: metadata/<date>/<kind>.jsonl, payloads/<date>/<kind>/<id>.json
type Writer struct {
	dir string
	cfg Config

	mu    sync.Mutex
	files map[string]*metaHandle // key: "kind:date"
}

type metaHandle struct {
	file *os.File
	date string
	kind string
}

func NewWriter(cfg Config) (*Writer, error) {
	if cfg.Dir == "" {
		cfg.Dir = "/var/log/zts/audits"
	}
	if cfg.MaxAgeDays <= 0 {
		cfg.MaxAgeDays = 30
	}

	for _, sub := range []string{"metadata", "payloads"} {
		if err := os.MkdirAll(filepath.Join(cfg.Dir, sub), 0700); err != nil {
			return nil, fmt.Errorf("audit: create %s dir: %w", sub, err)
		}
	}

	return &Writer{
		dir:   cfg.Dir,
		cfg:   cfg,
		files: make(map[string]*metaHandle),
	}, nil
}

func (w *Writer) WriteEvent(kind string, record audit.AuditRecord, rawPayload json.RawMessage) {
	date := record.AuditDate()
	id := record.AuditID()

	if err := w.writeMetadata(kind, date, record); err != nil {
		log.Printf("audit: failed to write %s metadata: %v", kind, err)
	}
	if err := w.writePayload(date, kind, id, rawPayload); err != nil {
		log.Printf("audit: failed to write %s payload: %v", kind, err)
	}
}

func (w *Writer) writeMetadata(kind, date string, record audit.AuditRecord) error {
	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal %s record: %w", kind, err)
	}
	line = append(line, '\n')

	w.mu.Lock()
	defer w.mu.Unlock()

	key := kind + ":" + date
	h, ok := w.files[key]
	if !ok {
		h, err = w.openMetaHandle(kind, date)
		if err != nil {
			return err
		}
		w.files[key] = h
	}

	if _, err = h.file.Write(line); err != nil {
		if closeErr := h.file.Close(); closeErr != nil {
			log.Printf("audit: failed to close %s metadata file: %v", key, closeErr)
		}
		delete(w.files, key)
		h, reopenErr := w.openMetaHandle(kind, date)
		if reopenErr != nil {
			return fmt.Errorf("reopen after write failure: %w (original: %v)", reopenErr, err)
		}
		w.files[key] = h
		if _, err = h.file.Write(line); err != nil {
			return fmt.Errorf("write after reopen: %w", err)
		}
	}

	return nil
}

func (w *Writer) openMetaHandle(kind, date string) (*metaHandle, error) {
	dayDir := filepath.Join(w.dir, "metadata", date)
	if err := os.MkdirAll(dayDir, 0700); err != nil {
		return nil, fmt.Errorf("create metadata dir %s: %w", dayDir, err)
	}

	path := filepath.Join(dayDir, kind+".jsonl")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return nil, fmt.Errorf("open metadata file %s: %w", path, err)
	}

	return &metaHandle{file: f, date: date, kind: kind}, nil
}

func (w *Writer) writePayload(date, kind, id string, rawPayload json.RawMessage) error {
	kindDir := filepath.Join(w.dir, "payloads", date, kind)
	if err := os.MkdirAll(kindDir, 0700); err != nil {
		return fmt.Errorf("create payload dir %s: %w", kindDir, err)
	}
	return os.WriteFile(filepath.Join(kindDir, id+".json"), rawPayload, 0600)
}

func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	var firstErr error
	for key, h := range w.files {
		if err := h.file.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		delete(w.files, key)
	}
	return firstErr
}

// Start runs periodic cleanup in the background until ctx is cancelled.
func (w *Writer) Start(ctx context.Context) {
	w.runCleanup()
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runCleanup()
		}
	}
}

func (w *Writer) runCleanup() {
	cutoff := time.Now().AddDate(0, 0, -w.cfg.MaxAgeDays)
	w.cleanDateDirs(filepath.Join(w.dir, "metadata"), cutoff)
	w.cleanDateDirs(filepath.Join(w.dir, "payloads"), cutoff)
}

func (w *Writer) cleanDateDirs(dir string, cutoff time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("audit: cleanup read dir %s: %v", dir, err)
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		t, err := time.Parse("2006-01-02", entry.Name())
		if err != nil {
			continue
		}
		if t.Before(cutoff) {
			path := filepath.Join(dir, entry.Name())
			if err := os.RemoveAll(path); err != nil {
				log.Printf("audit: cleanup remove %s: %v", path, err)
			} else {
				log.Printf("audit: cleaned up old directory %s", path)
			}
		}
	}
}
