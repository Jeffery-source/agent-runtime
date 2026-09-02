package task

import (
	"path/filepath"
	"testing"
	"time"
)

func TestFileStoreSaveLoadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "tasks.json")
	store := NewFileStore(path)

	now := time.Now()
	started := now.Add(time.Second)
	tasks := []*Task{
		{
			ID:        "t1",
			AgentID:   "a1",
			SessionID: "s1",
			Input:     "任务一",
			Status:    StatusCompleted,
			Output:    "完成",
			CreatedAt: now,
			StartedAt: &started,
		},
		{
			ID:        "t2",
			AgentID:   "a2",
			SessionID: "s2",
			Input:     "任务二",
			Status:    StatusPending,
			CreatedAt: now,
		},
	}

	if err := store.Save(tasks); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(loaded))
	}
	if loaded[0].ID != "t1" || loaded[0].Status != StatusCompleted ||
		loaded[0].Output != "完成" {
		t.Fatalf("unexpected task: %+v", loaded[0])
	}
	if loaded[0].StartedAt == nil {
		t.Fatal("expected started_at restored")
	}
	if loaded[1].ID != "t2" || loaded[1].Status != StatusPending {
		t.Fatalf("unexpected task: %+v", loaded[1])
	}
}

func TestFileStoreLoadMissingFile(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "missing.json"))

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded != nil {
		t.Fatalf("expected nil, got %v", loaded)
	}
}

func TestManagerRestore(t *testing.T) {
	m := NewManager()

	tasks := []*Task{
		{
			ID:        "t1",
			AgentID:   "a1",
			SessionID: "s1",
			Input:     "任务",
			Status:    StatusCompleted,
		},
		{ID: "", AgentID: "a2"}, // 空 ID 应被忽略
		nil,                      // nil 应被忽略
	}

	if err := m.Restore(tasks); err != nil {
		t.Fatalf("restore: %v", err)
	}

	got, err := m.Get("t1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != StatusCompleted {
		t.Fatalf("expected completed, got %s", got.Status)
	}

	if list := m.List(); len(list) != 1 {
		t.Fatalf("expected 1 restored task, got %d", len(list))
	}
}
