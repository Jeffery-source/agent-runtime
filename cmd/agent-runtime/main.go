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
	"github.com/Jeffery-source/agent-runtime/internal/skill"
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
	skills := skill.NewRegistry()
	if err := skills.Register(&skill.Skill{
		ID:          "demo_skill",
		Name:        "Demo Skill",
		Description: "用于测试 Agent Skill",
		Instructions: `
你正在使用 Demo Skill。

回答问题时：
1. 优先分析用户的问题
2. 如果需要实时信息，使用可用工具获取
3. 获取工具结果后再给出最终答案
`,
	}); err != nil {
		log.Fatalf("register skill: %v", err)
	}
	// 3. 模型客户端：优先真实 AI 网关，否则用演示模型。
	modelClient := newModelClient(cfg)

	// 4. Runtime 与任务服务（Worker 池）。
	rt := runtime.New(agents, sessions, modelClient, tools, taskManager)
	rt.SetSkillRegistry(skills)
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

	demoTools := []tool.Tool{
		&timeTool{},
		&MoneyTool{},
	}
	// 7. 注册演示工具与 Agent。

	for _, t := range demoTools {
		if err := tools.Register(t); err != nil {
			log.Fatalf("register tool %s: %v", t.Name(), err)
		}
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
			model.WithAPIKey(os.Getenv("AI_GATEWAY_API_KEY")),
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
		ID:           "demo-agent",
		Name:         "Demo Agent",
		Description:  "你是一个助手。当用户询问当前时间时，使用 get_time 工具获取时间。",
		Model:        "qwen3",
		SystemPrompt: "You are a helpful assistant.",
		Skills: []string{
			"demo_skill",
		},
		Tools:         []string{"get_time", "get_money"},
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
