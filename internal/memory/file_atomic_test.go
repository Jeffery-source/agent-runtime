package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Jeffery-source/agent-runtime/internal/message"
)

// TestFileStorePersistIsAtomic 验证消息落盘采用原子写：
// 不残留 .tmp，且重新打开后可完整恢复全部消息。
func TestFileStorePersistIsAtomic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "messages.json")

	store, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := store.Save(ctx, "s1", message.Message{
			Role:    message.RoleUser,
			Content: "你好",
		}); err != nil {
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

	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("reopen store: %v", err)
	}

	got, err := reopened.Get(ctx, "s1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}

	if len(got) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(got))
	}
}
