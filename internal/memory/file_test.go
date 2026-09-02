package memory

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/Jeffery-source/agent-runtime/internal/message"
)

func TestFileStoreSaveGet(t *testing.T) {
	ctx := context.Background()
	store, err := NewFileStore(filepath.Join(t.TempDir(), "messages.json"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	msg := message.Message{Role: message.RoleUser, Content: "你好"}
	if err := store.Save(ctx, "s1", msg); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.Get(ctx, "s1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
	if got[0].Content != "你好" {
		t.Fatalf("expected content 你好, got %q", got[0].Content)
	}
}

func TestFileStorePersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "messages.json")

	store, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	if err := store.Save(ctx, "s1", message.Message{
		Role:    message.RoleUser,
		Content: "第一条",
	}); err != nil {
		t.Fatalf("save: %v", err)
	}

	reopened, err := NewFileStore(path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got, err := reopened.Get(ctx, "s1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 message after reopen, got %d", len(got))
	}
	if got[0].Content != "第一条" {
		t.Fatalf("expected content 第一条, got %q", got[0].Content)
	}
}

func TestFileStoreGetUnknownSession(t *testing.T) {
	store, err := NewFileStore(filepath.Join(t.TempDir(), "messages.json"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}

	got, err := store.Get(context.Background(), "unknown")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d messages", len(got))
	}
}

func TestFileStoreContextCanceled(t *testing.T) {
	store, err := NewFileStore(filepath.Join(t.TempDir(), "messages.json"))
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if _, err := store.Get(ctx, "s1"); err == nil {
		t.Fatal("expected context error on Get")
	}
	if err := store.Save(ctx, "s1", message.Message{}); err == nil {
		t.Fatal("expected context error on Save")
	}
}

func TestInMemorySaveGet(t *testing.T) {
	ctx := context.Background()
	store := NewInMemory()

	msg := message.Message{
		Role:    message.RoleAssistant,
		Content: "回复",
		ToolCalls: []message.ToolCall{
			{ID: "c1", Name: "get_time"},
		},
	}
	if err := store.Save(ctx, "s1", msg); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := store.Get(ctx, "s1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 message, got %d", len(got))
	}
	if len(got[0].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(got[0].ToolCalls))
	}
}
