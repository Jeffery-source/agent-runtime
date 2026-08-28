package session

import (
	"testing"

	"github.com/Jeffery-source/agent-runtime/internal/message"
)

func TestManagerCreateAndGet(t *testing.T) {
	manager := NewManager()

	session, err := manager.Create(
		"session-001",
		"attendance",
		"user-001",
	)

	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	if session.ID != "session-001" {
		t.Fatalf("expected session ID %q, got %q",
			"session-001",
			session.ID,
		)
	}

	if session.AgentID != "attendance" {
		t.Fatalf("expected agent ID %q, got %q",
			"attendance",
			session.AgentID,
		)
	}

	got, err := manager.Get("session-001")
	if err != nil {
		t.Fatalf("get session failed: %v", err)
	}

	if got.ID != session.ID {
		t.Fatalf("expected session ID %q, got %q",
			session.ID,
			got.ID,
		)
	}
}

func TestManagerGetNotFound(t *testing.T) {
	manager := NewManager()

	_, err := manager.Get("not-exist")

	if err != ErrSessionNotFound {
		t.Fatalf(
			"expected ErrSessionNotFound, got %v",
			err,
		)
	}
}

func TestManagerAddMessage(t *testing.T) {
	manager := NewManager()

	_, err := manager.Create(
		"session-001",
		"attendance",
		"user-001",
	)

	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	msg := message.Message{
		Role:    message.RoleUser,
		Content: "张三今天有没有迟到？",
	}

	err = manager.AddMessage("session-001", msg)
	if err != nil {
		t.Fatalf("add message failed: %v", err)
	}

	session, err := manager.Get("session-001")
	if err != nil {
		t.Fatalf("get session failed: %v", err)
	}

	if len(session.Messages) != 1 {
		t.Fatalf(
			"expected 1 message, got %d",
			len(session.Messages),
		)
	}

	if session.Messages[0].Content != msg.Content {
		t.Fatalf(
			"expected message %q, got %q",
			msg.Content,
			session.Messages[0].Content,
		)
	}
}

func TestManagerDelete(t *testing.T) {
	manager := NewManager()

	_, err := manager.Create(
		"session-001",
		"attendance",
		"user-001",
	)

	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	err = manager.Delete("session-001")
	if err != nil {
		t.Fatalf("delete session failed: %v", err)
	}

	_, err = manager.Get("session-001")

	if err != ErrSessionNotFound {
		t.Fatalf(
			"expected ErrSessionNotFound after delete, got %v",
			err,
		)
	}
}

func TestManagerList(t *testing.T) {
	manager := NewManager()

	_, err := manager.Create(
		"session-001",
		"attendance",
		"user-001",
	)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	_, err = manager.Create(
		"session-002",
		"mes",
		"user-002",
	)
	if err != nil {
		t.Fatalf("create session failed: %v", err)
	}

	sessions := manager.List()

	if len(sessions) != 2 {
		t.Fatalf(
			"expected 2 sessions, got %d",
			len(sessions),
		)
	}
}
