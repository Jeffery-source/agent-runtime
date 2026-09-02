package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/Jeffery-source/agent-runtime/internal/agent"
	"github.com/Jeffery-source/agent-runtime/internal/config"
	"github.com/Jeffery-source/agent-runtime/internal/memory"
	"github.com/Jeffery-source/agent-runtime/internal/model"
	"github.com/Jeffery-source/agent-runtime/internal/runtime"
	"github.com/Jeffery-source/agent-runtime/internal/session"
	"github.com/Jeffery-source/agent-runtime/internal/task"
	"github.com/Jeffery-source/agent-runtime/internal/tool"
	"github.com/Jeffery-source/agent-runtime/internal/transport"
)

func main() {
	log.Println("Agent Runtime starting")

	// 1. 加载配置。
	cfg, err := config.Load("configs/config.json")
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	// 2. 领域组件（装配点 / composition root）。
	agents := agent.NewRegistry()
	sessions := session.NewManager()
	tools := tool.NewRegistry()
	taskManager := task.NewManager()

	// 3. 模型客户端：优先真实 AI 网关，否则用演示模型。
	modelClient := newModelClient(cfg)

	// 4. Runtime 与任务服务（Worker 池）。
	rt := runtime.New(agents, sessions, modelClient, tools, taskManager)
	executor := runtime.NewExecutor(taskManager, rt)
	taskService := task.NewServiceWithPool(
		taskManager,
		executor,
		cfg.Workers,
		cfg.QueueSize,
	)

	// 5. 记忆持久化（可插拔）。
	if mem := newMemory(cfg); mem != nil {
		rt.SetMemory(mem)
	}

	// 6. 任务恢复。
	taskStore := task.NewFileStore(
		filepath.Join(cfg.DataDir, "tasks.json"),
	)
	restoreTasks(taskManager, taskStore)

	// 7. 注册演示工具与 Agent。
	if err := tools.Register(&timeTool{}); err != nil {
		log.Fatalf("register tool: %v", err)
	}
	if err := agents.Register(demoAgent()); err != nil {
		log.Fatalf("register agent: %v", err)
	}

	// 8. 演示模式下跑一次同步闭环。
	if cfg.Gateway.BaseURL == "" {
		runDemo(rt, sessions, taskService)
	}

	// 9. 启动 HTTP 服务。
	handler := transport.NewServer(taskService, sessions).Handler()
	srv := &http.Server{
		Addr:    cfg.Server.Addr,
		Handler: handler,
	}

	go func() {
		log.Printf("HTTP server listening on %s", cfg.Server.Addr)
		if err := srv.ListenAndServe(); err != nil &&
			!errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	// 10. 优雅退出。
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	log.Println("shutting down...")

	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown: %v", err)
	}

	taskService.Shutdown()
	saveTasks(taskManager, taskStore)

	log.Println("Agent Runtime stopped")
}

func newModelClient(cfg *config.Config) model.Client {
	if cfg.Gateway.BaseURL != "" {
		log.Printf("using AI gateway: %s", cfg.Gateway.BaseURL)
		return model.NewGatewayClient(
			cfg.Gateway.BaseURL,
			model.WithTimeout(cfg.GatewayTimeout()),
			model.WithRetries(cfg.Gateway.Retries),
		)
	}

	log.Println("using demo model (set gateway.base_url to use a real gateway)")
	return &demoModel{}
}

func newMemory(cfg *config.Config) memory.Memory {
	if cfg.DataDir == "" {
		return nil
	}

	store, err := memory.NewFileStore(
		filepath.Join(cfg.DataDir, "messages.json"),
	)
	if err != nil {
		log.Printf("init memory file store: %v (falling back to none)", err)
		return nil
	}

	return store
}

func demoAgent() *agent.Agent {
	return &agent.Agent{
		ID:            "demo-agent",
		Name:          "Demo Agent",
		Description:   "演示用 Agent",
		Model:         "demo-model",
		SystemPrompt:  "You are a helpful assistant.",
		Tools:         []string{"get_time"},
		MaxIterations: 5,
	}
}

func runDemo(
	rt *runtime.Runtime,
	sessions *session.Manager,
	taskService task.TaskService,
) {

	if _, err := sessions.Create(
		"demo-session",
		"demo-agent",
		"user-1",
	); err != nil {
		log.Printf("create demo session: %v", err)
		return
	}

	resp, err := rt.Run(context.Background(), runtime.RunRequest{
		AgentID:   "demo-agent",
		SessionID: "demo-session",
		Input:     "现在几点？",
	})
	if err != nil {
		log.Printf("sync run error: %v", err)
	} else {
		log.Printf("sync run: status=%s content=%q", resp.Status, resp.Content)
	}
}

func restoreTasks(manager *task.Manager, store *task.FileStore) {
	tasks, err := store.Load()
	if err != nil {
		log.Printf("load tasks: %v", err)
		return
	}
	if len(tasks) == 0 {
		return
	}
	if err := manager.Restore(tasks); err != nil {
		log.Printf("restore tasks: %v", err)
		return
	}
	log.Printf("restored %d tasks", len(tasks))
}

func saveTasks(manager *task.Manager, store *task.FileStore) {
	tasks := manager.List()
	if err := store.Save(tasks); err != nil {
		log.Printf("save tasks: %v", err)
		return
	}
	log.Printf("saved %d tasks", len(tasks))
}
