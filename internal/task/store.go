package task

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// List 返回所有任务的副本。
func (m *Manager) List() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]*Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		out = append(out, cloneTask(t))
	}

	return out
}

// Restore 批量恢复任务（用于启动时从持久化后端加载）。
func (m *Manager) Restore(tasks []*Task) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, t := range tasks {
		if t == nil || t.ID == "" {
			continue
		}
		m.tasks[t.ID] = cloneTask(t)
	}

	return nil
}

// FileStore 把任务列表持久化到 JSON 文件，实现任务可恢复。
type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (f *FileStore) Save(tasks []*Task) error {
	data, err := json.MarshalIndent(tasks, "", "  ")
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

// writeFileAtomic 先写同目录临时文件，再 rename 原子替换目标文件。
// 这样即使进程在写入过程中被中断，目标文件仍保持上一次的完整内容，
// 不会出现半截 JSON 导致下次启动恢复失败。
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

func (f *FileStore) Load() ([]*Task, error) {
	data, err := os.ReadFile(f.path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var tasks []*Task
	if err := json.Unmarshal(data, &tasks); err != nil {
		return nil, err
	}

	return tasks, nil
}
