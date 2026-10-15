//go:build integration

package opensearch_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/faustbrian/go-search"
)

const maximumRebuildCheckpointBytes = 16 << 10

type durableRebuildCheckpoint struct {
	SnapshotID     string `json:"snapshot_id"`
	SourceCursor   string `json:"source_cursor"`
	SourceComplete bool   `json:"source_complete"`
	OutboxCursor   int    `json:"outbox_cursor"`
}

// This fixture's transaction is one fsync/rename of the authoritative state.
// Snapshot and high-water are captured together; each later source change and
// its commit-ordered replay intent are persisted together by Put/Delete.
type durableProjectionCommit struct {
	Revision uint64                       `json:"revision"`
	Deleted  bool                         `json:"deleted,omitempty"`
	Document durableAuthoritativeDocument `json:"document"`
}

func (commit durableProjectionCommit) operation(tenant, index string) search.WriteOperation {
	if commit.Deleted {
		return search.DeleteDocument(tenant, index, commit.Document.ID, commit.Document.Version)
	}
	return search.WriteOperation{Action: search.ActionIndex, Tenant: tenant, Index: index, ID: commit.Document.ID, Version: commit.Document.Version, Source: commit.Document.Source}
}

func (store *durableAuthoritativeStore) captureRebuildSnapshot(id string) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	state, err := store.loadLocked()
	if err != nil {
		return err
	}
	if id == "" || state.SnapshotID != "" {
		return errors.New("rebuild snapshot must be captured exactly once")
	}
	state.SnapshotID, state.SnapshotRevision = id, state.Revision
	state.Snapshot = append([]durableAuthoritativeDocument(nil), state.Documents...)
	return store.saveLocked(state)
}

type rebuildProcessConfig struct {
	Endpoint, SourcePath, CheckpointPath, SnapshotID string
	Tenant, Index, Alias, Physical, Fingerprint      string
	StopAfter                                        string
}

func TestDurableRebuildCaptureKeepsSnapshotAndPostBoundaryCommits(t *testing.T) {
	limits := search.DefaultLimits()
	original, err := search.NewDocument("tenant", "records", "original", 1, json.RawMessage(`{"value":"old"}`), limits)
	if err != nil {
		t.Fatal(err)
	}
	store, err := newDurableAuthoritativeStore(t.TempDir(), original.Tenant, original.Index, []search.Document{original}, limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.captureRebuildSnapshot("snapshot-1"); err != nil {
		t.Fatal(err)
	}
	added, err := search.NewDocument(original.Tenant, original.Index, "added", 2, json.RawMessage(`{"value":"new"}`), limits)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Put(t.Context(), added); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(t.Context(), original.ID, 3); err != nil {
		t.Fatal(err)
	}
	reopened := &durableAuthoritativeStore{path: store.path, tenant: store.tenant, index: store.index, limits: limits, snapshot: true}
	state, err := reopened.loadLocked()
	if err != nil || state.SnapshotID != "snapshot-1" || state.SnapshotRevision != 1 || len(state.Outbox) != 2 ||
		state.Outbox[0].Revision != 2 || state.Outbox[0].Document.ID != added.ID ||
		state.Outbox[1].Revision != 3 || !state.Outbox[1].Deleted || state.Outbox[1].Document.ID != original.ID ||
		state.Tombstones[original.ID] != 3 {
		t.Fatalf("reopened capture lost its boundary or committed replay: %#v/%v", state, err)
	}
	if operation := state.Outbox[1].operation(store.tenant, store.index); len(operation.Source) != 0 || operation.Action != search.ActionDelete {
		t.Fatal("persisted tombstone must reconstruct a source-free delete")
	}
	page, err := reopened.Read(t.Context(), original.Tenant, original.Index, "", 1)
	if err != nil || !page.Done || len(page.Records) != 1 || page.Records[0].ID != original.ID || page.Records[0].Version != original.Version {
		t.Fatalf("snapshot changed with live source: %#v/%v", page, err)
	}
	if err := reopened.captureRebuildSnapshot("snapshot-2"); err == nil {
		t.Fatal("replacement must not silently capture a fresh snapshot")
	}
	checkpointPath := filepath.Join(filepath.Dir(store.path), "rebuild-checkpoint.json")
	if err := saveDurableRebuildCheckpoint(checkpointPath, durableRebuildCheckpoint{SnapshotID: state.SnapshotID, SourceComplete: true, OutboxCursor: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := loadDurableRebuildCheckpoint(checkpointPath, "snapshot-2"); err == nil {
		t.Fatal("replacement accepted a checkpoint from a different snapshot")
	}
}

// runRebuildProcess waits for the durable checkpoint barrier before killing the
// worker. A replacement receives paths/identities only, never parent heap state.
func runRebuildProcess(t *testing.T, config rebuildProcessConfig, stopAfter string) int {
	t.Helper()
	config.StopAfter = stopAfter
	encoded, err := json.Marshal(config)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestRebuildProcessWorker$", "-test.timeout=80s")
	command.Env = append(os.Environ(), "GOLIB_REBUILD_WORKER="+string(encoded))
	var diagnostics bytes.Buffer
	command.Stderr = &diagnostics
	stdout, err := command.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	}()
	pid := command.Process.Pid
	if pid == os.Getpid() {
		t.Fatal("rebuild worker must run outside the parent process")
	}
	scanner := bufio.NewScanner(stdout)
	barrier := fmt.Sprintf("rebuild-checkpoint:%s:%d", stopAfter, pid)
	stopped := false
	for scanner.Scan() {
		if stopAfter != "" && scanner.Text() == barrier {
			if err := command.Process.Kill(); err != nil {
				t.Fatalf("kill checkpointed rebuild worker: %v", err)
			}
			stopped = true
			break
		}
		diagnostics.WriteString(scanner.Text() + "\n")
	}
	waitErr := command.Wait()
	waited = true
	if scanner.Err() != nil || ctx.Err() != nil || stopAfter != "" && !stopped || stopAfter == "" && waitErr != nil || stopped && waitErr == nil {
		t.Fatalf("rebuild worker failed: stop=%s stopped=%t wait=%v scan=%v deadline=%v\n%s", stopAfter, stopped, waitErr, scanner.Err(), ctx.Err(), diagnostics.String())
	}
	t.Logf("rebuild process %d: checkpoint=%s killed=%t", pid, stopAfter, stopped)
	return pid
}

func TestRebuildProcessWorker(t *testing.T) {
	encoded := os.Getenv("GOLIB_REBUILD_WORKER")
	if encoded == "" {
		t.Skip("started only by the bounded real-backend rebuild scenario")
	}
	var config rebuildProcessConfig
	if len(encoded) > maximumRebuildCheckpointBytes || json.Unmarshal([]byte(encoded), &config) != nil || config.SnapshotID == "" {
		t.Fatal("invalid rebuild worker configuration")
	}
	store := &durableAuthoritativeStore{
		path: config.SourcePath, tenant: config.Tenant, index: config.Index,
		limits: search.DefaultLimits(), snapshot: true,
	}
	state, err := store.loadLocked()
	if err != nil || state.SnapshotID != config.SnapshotID || state.SnapshotRevision == 0 {
		t.Fatalf("persisted source boundary unavailable: %v", err)
	}
	checkpoint, err := loadDurableRebuildCheckpoint(config.CheckpointPath, state.SnapshotID)
	if err != nil || checkpoint.OutboxCursor > len(state.Outbox) {
		t.Fatalf("persisted rebuild checkpoint unavailable: %v", err)
	}
	client := newBoundIntegrationSearchClient(t, config.Endpoint, config.Tenant, config.Index, config.Alias, config.Physical, config.Fingerprint, store.limits)
	stop := func(stage string) {
		if config.StopAfter == stage {
			fmt.Printf("rebuild-checkpoint:%s:%d\n", stage, os.Getpid())
			// The parent kills us; this bound prevents an orphan waiting forever.
			time.Sleep(time.Minute)
			t.Fatal("parent did not kill checkpointed rebuild worker")
		}
	}
	for !checkpoint.SourceComplete {
		page, readErr := store.Read(t.Context(), config.Tenant, config.Index, checkpoint.SourceCursor, 1)
		if readErr != nil {
			t.Fatal(readErr)
		}
		operations := make([]search.WriteOperation, len(page.Records))
		for position, record := range page.Records {
			operations[position] = search.IndexDocument(*record.Document)
		}
		if len(operations) != 0 {
			result, bulkErr := client.Bulk(t.Context(), search.BulkRequest{Operations: operations, Refresh: search.RefreshWaitFor})
			if bulkErr != nil || result.Partial() {
				t.Fatalf("rebuild source page: %v", bulkErr)
			}
		}
		checkpoint.SourceCursor, checkpoint.SourceComplete = page.Cursor, page.Done
		if err := saveDurableRebuildCheckpoint(config.CheckpointPath, checkpoint); err != nil {
			t.Fatal(err)
		}
		stop("scan")
	}
	for checkpoint.OutboxCursor < len(state.Outbox) {
		commit := state.Outbox[checkpoint.OutboxCursor]
		if commit.Revision <= state.SnapshotRevision || checkpoint.OutboxCursor > 0 && commit.Revision <= state.Outbox[checkpoint.OutboxCursor-1].Revision {
			t.Fatal("replay must follow the captured commit-order boundary")
		}
		outcome, replayErr := client.Write(t.Context(), commit.operation(config.Tenant, config.Index), search.RefreshWaitFor)
		if replayErr != nil || outcome.State != search.OutcomeApplied {
			t.Fatalf("rebuild replay: state=%v error=%v", outcome.State, replayErr)
		}
		checkpoint.OutboxCursor++
		if err := saveDurableRebuildCheckpoint(config.CheckpointPath, checkpoint); err != nil {
			t.Fatal(err)
		}
		stop("replay")
	}
}

func loadDurableRebuildCheckpoint(path, snapshotID string) (durableRebuildCheckpoint, error) {
	file, err := os.Open(path)
	if err != nil {
		return durableRebuildCheckpoint{}, err
	}
	defer func() { _ = file.Close() }()
	encoded, err := io.ReadAll(io.LimitReader(file, maximumRebuildCheckpointBytes+1))
	if err != nil || len(encoded) > maximumRebuildCheckpointBytes {
		return durableRebuildCheckpoint{}, errors.New("rebuild checkpoint exceeds its byte limit")
	}
	var checkpoint durableRebuildCheckpoint
	decoder := json.NewDecoder(bytes.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&checkpoint) != nil || decoder.Decode(&struct{}{}) != io.EOF || checkpoint.SnapshotID != snapshotID || checkpoint.OutboxCursor < 0 {
		return durableRebuildCheckpoint{}, errors.New("rebuild checkpoint is malformed or belongs to another snapshot")
	}
	return checkpoint, nil
}

func saveDurableRebuildCheckpoint(path string, checkpoint durableRebuildCheckpoint) error {
	if checkpoint.SnapshotID == "" || checkpoint.OutboxCursor < 0 {
		return errors.New("rebuild checkpoint is invalid")
	}
	encoded, err := json.Marshal(checkpoint)
	if err != nil || len(encoded) > maximumRebuildCheckpointBytes {
		return errors.New("rebuild checkpoint encoding exceeds its byte limit")
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".rebuild-checkpoint-*.tmp")
	if err != nil {
		return err
	}
	temporaryName := temporary.Name()
	committed := false
	defer func() {
		_ = temporary.Close()
		if !committed {
			_ = os.Remove(temporaryName)
		}
	}()
	if _, err := temporary.Write(encoded); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return err
	}
	committed = true
	directoryHandle, err := os.Open(directory)
	if err != nil {
		return err
	}
	defer func() { _ = directoryHandle.Close() }()
	return directoryHandle.Sync()
}
