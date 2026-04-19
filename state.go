package orc

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	StateFile    = "orc.state.json"
	HistoryFile  = ".orc/runs.ndjson"
	stateVersion = 1
)

type TaskRecord struct {
	Status   string        `json:"status"`
	Started  time.Time     `json:"started,omitempty"`
	Finished time.Time     `json:"finished,omitempty"`
	Duration time.Duration `json:"duration,omitempty"`
	Err      string        `json:"err,omitempty"`
}

type RunRecord struct {
	ID       string                `json:"id"`
	Version  int                   `json:"version"`
	Runbook  string                `json:"runbook"`
	Plan     string                `json:"plan"`
	Commit   string                `json:"commit"`
	Started  time.Time             `json:"started"`
	Finished time.Time             `json:"finished"`
	Duration time.Duration         `json:"duration"`
	Status   RunStatus             `json:"status"`
	Tasks    map[string]TaskRecord `json:"tasks"`
}

func LoadState() (*RunRecord, error) {
	data, err := os.ReadFile(StateFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read state file: %w", err)
	}
	var record RunRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return nil, fmt.Errorf("unmarshal state file: %w", err)
	}
	return &record, nil
}

func LoadHistory(n int) ([]RunRecord, error) {
	f, err := os.Open(HistoryFile)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read history file: %w", err)
	}
	defer f.Close()

	var records []RunRecord
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		var record RunRecord
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			return nil, fmt.Errorf("unmarshal history file: %w", err)
		}
		records = append(records, record)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan history file: %w", err)
	}

	if n > 0 && len(records) > n {
		records = records[len(records)-n:]
	}

	return records, nil
}

func (rb *Runbook) buildRunRecord(
	plan string,
	started, finished time.Time,
	states map[*Task]*taskState,
) RunRecord {

	var status RunStatus
	for _, s := range states {
		if s.status == StatusFailed {
			status = RunFailed
			break
		}
		if s.status == StatusConfirmFailed && status != RunFailed {
			status = RunConfirmFailed
		}
		if s.status == StatusCancelled && status != RunFailed && status != RunConfirmFailed {
			status = RunCancelled
		}
	}
	if status == "" {
		status = RunSucceeded
	}

	tasks := make(map[string]TaskRecord, len(states))
	for _, s := range states {
		tasks[s.task.name] = TaskRecord{
			Status:   s.status.String(),
			Started:  s.started,
			Finished: s.finished,
			Duration: s.duration,
			Err:      errString(s.err),
		}
	}

	return RunRecord{
		ID:       generateRunID(started),
		Version:  stateVersion,
		Runbook:  rb.name,
		Plan:     plan,
		Commit:   gitCommit(),
		Started:  started,
		Finished: finished,
		Duration: finished.Sub(started),
		Tasks:    tasks,
		Status:   status,
	}
}

func persistRecord(record RunRecord) error {
	if err := writeState(record); err != nil {
		return fmt.Errorf("write state: %w", err)
	}
	if err := appendHistory(record); err != nil {
		return fmt.Errorf("append history: %w", err)
	}
	return nil
}

func writeState(record RunRecord) error {
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(StateFile)
	tmp, err := os.CreateTemp(dir, ".orc.state.*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, StateFile); err != nil {
		return err
	}
	return nil
}

func appendHistory(record RunRecord) error {
	if err := os.MkdirAll(filepath.Dir(HistoryFile), 0755); err != nil {
		return fmt.Errorf("create history dir: %w", err)
	}
	line, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshal run record: %w", err)
	}
	f, err := os.OpenFile(HistoryFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open history file: %w", err)
	}
	defer f.Close()
	_, err = f.Write(append(line, '\n'))
	if err != nil {
		return fmt.Errorf("write history file: %w", err)
	}
	return nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func gitCommit() string {
	out, err := exec.Command("git", "describe", "--always", "--dirty").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func generateRunID(t time.Time) string {
	return t.Format("20060102-150405.000")
}

func LoadRecord() (*RunRecord, error) {
	records, err := LoadHistory(1)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return &records[0], nil
}

func LoadRecordByID(id string) (*RunRecord, error) {
	records, err := LoadHistory(0)
	if err != nil {
		return nil, err
	}
	for _, r := range records {
		if r.ID == id {
			return &r, nil
		}
	}
	return nil, fmt.Errorf("run record with ID %q not found", id)
}
