package audit

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/config"
)

// Writer writes audit records to local files.
// Metadata records go to metadata/<date>.jsonl (one line per event).
// Full payloads go to payloads/<date>/<id>.json (one file per event).
// All date directories use UTC.
type Writer struct {
	dir string
	cfg config.AuditConfig

	mu          sync.Mutex
	currentDate string
	metaFile    *os.File
}

// NewWriter creates a new audit Writer. It ensures the base directories exist.
func NewWriter(cfg config.AuditConfig) (*Writer, error) {
	metaDir := filepath.Join(cfg.Dir, "metadata")
	payloadsDir := filepath.Join(cfg.Dir, "payloads")

	if err := os.MkdirAll(metaDir, 0700); err != nil {
		return nil, fmt.Errorf("audit: create metadata dir: %w", err)
	}
	if err := os.MkdirAll(payloadsDir, 0700); err != nil {
		return nil, fmt.Errorf("audit: create payloads dir: %w", err)
	}

	return &Writer{
		dir: cfg.Dir,
		cfg: cfg,
	}, nil
}

// Write persists an audit record (metadata entry) and the full raw payload.
// This is safe for concurrent use.
func (w *Writer) Write(record Record, rawPayload json.RawMessage) {
	date := DateFromEpochMs(record.StartTs)

	// Write metadata entry
	if err := w.writeMetadata(date, record); err != nil {
		log.Printf("audit: failed to write metadata: %v", err)
	}

	// Write payload file
	if err := w.writePayload(date, record.ID, rawPayload); err != nil {
		log.Printf("audit: failed to write payload: %v", err)
	}
}

// writeMetadata appends a JSON line to the daily metadata file.
func (w *Writer) writeMetadata(date string, record Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Open or rotate file if needed (date changed, first write, or previous handle gone stale)
	if w.currentDate != date || w.metaFile == nil {
		if err := w.openMetaFile(date); err != nil {
			return err
		}
	}

	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal metadata record: %w", err)
	}
	line = append(line, '\n')

	if _, err = w.metaFile.Write(line); err != nil {
		// Handle stale file handle (e.g. directory was deleted while running).
		// Close the broken handle and try once more with a fresh open.
		w.metaFile.Close()
		w.metaFile = nil
		if reopenErr := w.openMetaFile(date); reopenErr != nil {
			return fmt.Errorf("reopen after write failure: %w (original: %v)", reopenErr, err)
		}
		if _, err = w.metaFile.Write(line); err != nil {
			return fmt.Errorf("write after reopen: %w", err)
		}
	}

	return nil
}

// openMetaFile ensures the metadata directory exists and opens the daily file.
// Must be called with w.mu held.
func (w *Writer) openMetaFile(date string) error {
	if w.metaFile != nil {
		w.metaFile.Close()
		w.metaFile = nil
	}

	metaDir := filepath.Join(w.dir, "metadata")
	if err := os.MkdirAll(metaDir, 0700); err != nil {
		return fmt.Errorf("create metadata dir %s: %w", metaDir, err)
	}

	path := filepath.Join(metaDir, fmt.Sprintf("audit-%s.jsonl", date))
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("open metadata file %s: %w", path, err)
	}

	w.metaFile = f
	w.currentDate = date
	return nil
}

// writePayload writes the full raw request body to a per-event file.
func (w *Writer) writePayload(date, id string, rawPayload json.RawMessage) error {
	dayDir := filepath.Join(w.dir, "payloads", date)
	if err := os.MkdirAll(dayDir, 0700); err != nil {
		return fmt.Errorf("create payload dir %s: %w", dayDir, err)
	}

	path := filepath.Join(dayDir, id+".json")
	return os.WriteFile(path, rawPayload, 0600)
}

// Close closes any open file handles. Safe to call multiple times.
func (w *Writer) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.metaFile != nil {
		err := w.metaFile.Close()
		w.metaFile = nil
		return err
	}
	return nil
}

// RunCleanup removes audit files older than maxAgeDays.
// Should be called periodically (e.g. once per hour).
func (w *Writer) RunCleanup() {
	cutoff := time.Now().AddDate(0, 0, -w.cfg.MaxAgeDays)
	w.cleanDir(filepath.Join(w.dir, "metadata"), cutoff)
	w.cleanDir(filepath.Join(w.dir, "payloads"), cutoff)
}

func (w *Writer) cleanDir(dir string, cutoff time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("audit: cleanup read dir %s: %v", dir, err)
		return
	}

	for _, entry := range entries {
		name := entry.Name()
		var dateStr string
		if entry.IsDir() {
			// payloads/<date>/
			dateStr = name
		} else {
			// metadata/audit-<date>.jsonl
			if len(name) >= 16 && name[:6] == "audit-" {
				dateStr = name[6:16] // extract "2006-01-02"
			}
		}

		if dateStr == "" {
			continue
		}

		t, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		if t.Before(cutoff) {
			path := filepath.Join(dir, name)
			if entry.IsDir() {
				if err := os.RemoveAll(path); err != nil {
					log.Printf("audit: cleanup remove %s: %v", path, err)
				} else {
					log.Printf("audit: cleaned up old directory %s", path)
				}
			} else {
				if err := os.Remove(path); err != nil {
					log.Printf("audit: cleanup remove %s: %v", path, err)
				} else {
					log.Printf("audit: cleaned up old file %s", path)
				}
			}
		}
	}
}
