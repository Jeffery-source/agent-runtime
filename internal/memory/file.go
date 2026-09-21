package memory

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"github.com/Jeffery-source/agent-runtime/internal/message"
)

// FileStore 把会话消息持久化到单个 JSON 文件，可插拔替换 InMemory。
// 每次 Save 后全量落盘，进程重启后通过 NewFileStore 加载即可恢复会话历史。
type FileStore struct {
	mu       sync.Mutex
	path     string
	messages map[string][]message.Message
}

// NewFileStore 打开（或创建）指定路径的文件存储，并加载已有数据。
func NewFileStore(path string) (*FileStore, error) {
	fs := &FileStore{
		path:     path,
		messages: make(map[string][]message.Message),
	}

	if err := fs.load(); err != nil {
		return nil, err
	}

	return fs, nil
}

func (f *FileStore) Get(
	ctx context.Context,
	sessionID string,
) ([]message.Message, error) {

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	msgs := f.messages[sessionID]

	out := make([]message.Message, 0, len(msgs))
	for _, msg := range msgs {
		out = append(out, cloneMessage(msg))
	}

	return out, nil
}

func (f *FileStore) Save(
	ctx context.Context,
	sessionID string,
	msg message.Message,
) error {

	if err := ctx.Err(); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.messages[sessionID] = append(
		f.messages[sessionID],
		cloneMessage(msg),
	)

	return f.persist()
}

func (f *FileStore) load() error {
	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &f.messages)
}

func (f *FileStore) persist() error {
	data, err := json.MarshalIndent(f.messages, "", "  ")
	if err != nil {
		return err
	}

	if dir := filepath.Dir(f.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return writeFileAtomic(f.path, data)
}

// writeFileAtomic 先写同目录临时文件，再 rename 原子替换目标文件，
// 避免进程写入中途被中断时留下半截 JSON。
func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"

	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}

	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	return nil
}
