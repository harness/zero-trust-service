package audit

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Reader reads audit records from local metadata files and payload files.
type Reader struct {
	dir string
}

// NewReader creates a new audit Reader.
func NewReader(dir string) *Reader {
	return &Reader{dir: dir}
}

// List scans daily metadata files within the given time range and returns
// matching records. Filters are applied line-by-line without loading
// everything into memory.
func (rd *Reader) List(req ListRequest) (*ListResponse, error) {
	from := time.UnixMilli(req.FromMs).UTC()
	to := time.UnixMilli(req.ToMs).UTC()
	dates := datesToScan(from, to)

	var matched []Record
	total := 0

	for _, date := range dates {
		path := filepath.Join(rd.dir, "metadata", fmt.Sprintf("audit-%s.jsonl", date))

		records, count, err := scanMetadataFile(path, req)
		if err != nil {
			continue // file may not exist for that date, skip
		}
		total += count
		matched = append(matched, records...)
	}

	// Apply offset and limit on the aggregated results
	start := req.Offset
	if start > len(matched) {
		start = len(matched)
	}
	end := start + req.Limit
	if end > len(matched) {
		end = len(matched)
	}

	return &ListResponse{
		Audits: matched[start:end],
		Total:  total,
		Limit:  req.Limit,
		Offset: req.Offset,
	}, nil
}

// GetPayload reads the full raw payload for a specific audit record.
// Searches payloads directory from most recent date backwards.
func (rd *Reader) GetPayload(id string) (json.RawMessage, error) {
	payloadsDir := filepath.Join(rd.dir, "payloads")
	dateDirs, err := os.ReadDir(payloadsDir)
	if err != nil {
		return nil, fmt.Errorf("read payloads dir: %w", err)
	}

	for i := len(dateDirs) - 1; i >= 0; i-- {
		entry := dateDirs[i]
		if !entry.IsDir() {
			continue
		}
		path := filepath.Join(payloadsDir, entry.Name(), id+".json")
		data, err := os.ReadFile(path)
		if err == nil {
			return data, nil
		}
	}

	return nil, fmt.Errorf("payload %q not found", id)
}

// scanMetadataFile reads a single metadata file line-by-line, applying filters.
func scanMetadataFile(path string, req ListRequest) ([]Record, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	var matched []Record
	total := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var rec Record
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}

		if !matchesFilters(rec, req) {
			continue
		}

		total++
		matched = append(matched, rec)
	}

	return matched, total, scanner.Err()
}

// matchesFilters checks if a record matches all the provided filters.
func matchesFilters(rec Record, req ListRequest) bool {
	// Filter by epoch millis range
	if rec.StartTs < req.FromMs || rec.StartTs >= req.ToMs {
		return false
	}

	if req.AccountID != "" && rec.AccountID != req.AccountID {
		return false
	}
	if req.TaskType != "" && rec.TaskType != req.TaskType {
		return false
	}
	if req.TaskID != "" && rec.TaskID != req.TaskID {
		return false
	}
	if req.Allowed != nil && rec.Allowed != *req.Allowed {
		return false
	}

	return true
}

// datesToScan returns a list of UTC date strings (YYYY-MM-DD) between from and to (inclusive).
func datesToScan(from, to time.Time) []string {
	var dates []string
	current := from.UTC().Truncate(24 * time.Hour)
	end := to.UTC().Truncate(24 * time.Hour)

	for !current.After(end) {
		dates = append(dates, current.Format("2006-01-02"))
		current = current.AddDate(0, 0, 1)
	}
	return dates
}
