package task

import (
	"os"
	"path/filepath"
	"testing"
)

// TestFileStoreSaveIsAtomic 验证任务落盘采用「临时文件 + rename」的原子写：
// 反复覆盖写不会残留 .tmp，且目标文件始终是完整 JSON，可被重新加载。
func TestFileStoreSaveIsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.json")

	store := NewFileStore(path)

	tasks := []*Task{
		{
			ID:        "t1",
			AgentID:   "demo-agent",
			SessionID: "s1",
			Input:     "现在几点了",
			Status:    StatusCompleted,
		},
	}

	for i := 0; i < 3; i++ {
		if err := store.Save(tasks); err != nil {
			t.Fatalf("save #%d: %v", i, err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}

	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Fatalf(
				"atomic save should not leave temp file, got %q",
				e.Name(),
			)
		}
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("expected 1 task, got %d", len(loaded))
	}

	if loaded[0].ID != "t1" ||
		loaded[0].Status != StatusCompleted {
		t.Fatalf("unexpected task: %+v", loaded[0])
	}
}
