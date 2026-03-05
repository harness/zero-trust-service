package file

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"git0.harness.io/l7B_kbSEQD2wjrM7PShm5w/PROD/Harness_Commons/zero-trust-service/audit"
)

type Reader struct {
	dir string
}

func NewReader(dir string) *Reader {
	return &Reader{dir: dir}
}

func (rd *Reader) List(req ListRequest) (*ListResponse, error) {
	kind := req.Kind
	if kind == "" {
		kind = audit.EventVerify
	}

	dates := datesToScan(req.FromTime, req.ToTime)

	switch kind {
	case audit.EventOutput:
		return rd.listOutput(dates, req)
	default:
		return rd.listVerify(dates, req)
	}
}

func (rd *Reader) listVerify(dates []string, req ListRequest) (*ListResponse, error) {
	var matched []audit.Record
	total := 0

	for _, date := range dates {
		path := filepath.Join(rd.dir, "metadata", date, "verify.jsonl")
		records, count, err := scanVerifyFile(path, req)
		if err != nil {
			continue
		}
		total += count
		matched = append(matched, records...)
	}

	start, end := paginateRange(len(matched), req.Offset, req.Limit)
	return &ListResponse{
		Kind:   audit.EventVerify,
		Audits: matched[start:end],
		Total:  total,
		Limit:  req.Limit,
		Offset: req.Offset,
	}, nil
}

func (rd *Reader) listOutput(dates []string, req ListRequest) (*ListResponse, error) {
	var matched []audit.OutputRecord
	total := 0

	for _, date := range dates {
		path := filepath.Join(rd.dir, "metadata", date, "output.jsonl")
		records, count, err := scanOutputFile(path, req)
		if err != nil {
			continue
		}
		total += count
		matched = append(matched, records...)
	}

	start, end := paginateRange(len(matched), req.Offset, req.Limit)
	return &ListResponse{
		Kind:   audit.EventOutput,
		Audits: matched[start:end],
		Total:  total,
		Limit:  req.Limit,
		Offset: req.Offset,
	}, nil
}

// GetPayload searches all payload kind subdirectories for the given ID.
func (rd *Reader) GetPayload(id string) (json.RawMessage, error) {
	payloadsDir := filepath.Join(rd.dir, "payloads")
	dateDirs, err := os.ReadDir(payloadsDir)
	if err != nil {
		return nil, fmt.Errorf("read payloads dir: %w", err)
	}

	for i := len(dateDirs) - 1; i >= 0; i-- {
		dateEntry := dateDirs[i]
		if !dateEntry.IsDir() {
			continue
		}
		kindDirs, err := os.ReadDir(filepath.Join(payloadsDir, dateEntry.Name()))
		if err != nil {
			continue
		}
		for _, kindEntry := range kindDirs {
			if !kindEntry.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(payloadsDir, dateEntry.Name(), kindEntry.Name(), id+".json"))
			if err == nil {
				return data, nil
			}
		}
	}

	return nil, fmt.Errorf("payload %q not found", id)
}

func scanVerifyFile(path string, req ListRequest) ([]audit.Record, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	var matched []audit.Record
	total := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec audit.Record
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if !matchesVerifyFilters(rec, req) {
			continue
		}
		total++
		matched = append(matched, rec)
	}

	return matched, total, scanner.Err()
}

func scanOutputFile(path string, req ListRequest) ([]audit.OutputRecord, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer f.Close()

	var matched []audit.OutputRecord
	total := 0

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 256*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec audit.OutputRecord
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		if !matchesOutputFilters(rec, req) {
			continue
		}
		total++
		matched = append(matched, rec)
	}

	return matched, total, scanner.Err()
}

func matchesVerifyFilters(rec audit.Record, req ListRequest) bool {
	if rec.StartTime.Before(req.FromTime) || rec.StartTime.After(req.ToTime) {
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

func matchesOutputFilters(rec audit.OutputRecord, req ListRequest) bool {
	ts := time.UnixMilli(rec.Timestamp).UTC()
	if ts.Before(req.FromTime) || ts.After(req.ToTime) {
		return false
	}
	if req.AccountID != "" && rec.AccountID != req.AccountID {
		return false
	}
	if req.TaskID != "" && rec.TaskID != req.TaskID {
		return false
	}
	return true
}

func paginateRange(total, offset, limit int) (int, int) {
	start := offset
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}
	return start, end
}

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
