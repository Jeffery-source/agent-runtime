# Agent Runtime

企业级 AI Agent 执行引擎（Go）。

Agent Runtime 解决的是「**Agent 怎么跑起来**」这件事：当上层接入方丢进来一句用户输入，如何驱动一个「会思考、能调用工具、记得住上下文」的 Agent 把任务跑完——并以**可异步、可取消、可恢复、可对外服务**的工程形态交付。

它**不负责**「做什么业务」：不含任何 Agent 业务逻辑、不实现具体工具、不定义对外协议规范（HTTP 仅是示例传输层）、不做模型推理、不做多租户鉴权。这些能力全部通过**接口**在装配阶段注入。

---

## 目录

- [特性](#特性)
- [架构总览](#架构总览)
  - [职责边界](#职责边界)
  - [分层与依赖](#分层与依赖)
  - [领域模型](#领域模型)
  - [组合根（main.go）](#组合根maingo)
  - [一次任务的运行时数据流](#一次任务的运行时数据流)
  - [关键设计决策](#关键设计决策)
- [目录结构](#目录结构)
- [快速开始](#快速开始)
- [HTTP API](#http-api)
- [配置说明](#配置说明)
- [扩展指南](#扩展指南)
  - [新增一个 Agent](#新增一个-agent)
  - [实现并注册一个工具](#实现并注册一个工具)
  - [接入真实模型网关](#接入真实模型网关)
  - [更换持久化后端](#更换持久化后端)
- [测试](#测试)
- [常见问题](#常见问题)

---

## 特性

| 能力 | 说明 |
|---|---|
| **Agent Loop** | 「模型调用 → 工具调用 → 结果回写会话 → 再次调用」多轮循环，带 `MaxIterations` 上限与 context 取消 |
| **任务状态机** | `pending → running → completed / failed / canceled`，所有流转带守卫，非法流转报错 |
| **异步调度** | 有界队列 + Worker 池；`Submit` 立即返回，队列满返回 `ErrQueueFull` 形成背压 |
| **任务取消** | 通过 context 全链路传导，只对 `running` 任务生效，无轮询无状态残留 |
| **可插拔模型** | `model.Client` 接口；内置 `GatewayClient`（超时 / 重试 / 工具定义下发）与演示模型 `demoModel` |
| **可插拔工具** | `tool.Tool` 接口 + 注册表；Agent 通过白名单声明可用工具 |
| **会话与记忆** | `Session` 保存完整消息历史；`memory.Memory` 接口，内置内存与 JSON 文件两种实现 |
| **可恢复** | 任务与会话消息按 JSON 落盘，进程重启后自动恢复 |
| **HTTP 传输层** | 会话创建、任务提交 / 查询 / 取消四个端点，错误映射为语义化状态码 |
| **配置驱动** | JSON 配置文件 + 默认值回退；零配置即可启动 |
| **并发安全** | Registry / Manager 内部 `RWMutex`，对外返回深拷贝，已通过 `-race` 验证 |

---

## 架构总览

### 职责边界

```
上层接入方：业务调用方 / HTTP API / transport
        │  提交"运行一次 Agent"的请求
        ▼
┌─────────────────── Agent Runtime 引擎 ───────────────────┐
│   编排层  runtime：Agent Loop、上下文组装、状态推进      │
│   任务层  task：有界队列、Worker 池、状态机、任务恢复    │
│   领域层  domain：agent/session/message/tool/model/memory │
└───────────────────┬─────────────────────────────────────┘
        │  调用接口（只认抽象，不认识具体实现）
        ▼
下层基础设施：AI 网关 · 工具执行 · 消息/任务持久化
```

### 分层与依赖

代码组织结构为**单向依赖、上层依赖下层**：

```
cmd（组合根 / 装配入口）
  │
  ▼
transport（HTTP 传输层）        runtime（Agent Loop + Executor）
  └──────────────┬───────────────────┘
                 ▼
task（Service / Manager / Worker / 状态机 / 任务存储）
                 ▼
domain：agent · session · message · tool · model · memory · agentcontext
                 ▼
基础设施：config · AI 网关客户端 · JSON 文件存储
```

依赖规则（硬性约束）：

1. 依赖方向单向向下，`domain` 不得反向依赖 `runtime` / `task`；
2. `cmd` 是**唯一**的装配点（composition root），全工程只有这里有 `new` 与注入；
3. **接口定义在"使用者"一侧，实现反向注入**。例如 `task.Executor` 接口定义在 `task` 包，`runtime.Executor` 只是恰好实现了它——因此 `task` 包不依赖 `runtime`，而运行时的调用方向却是 `task → runtime`，编译与运行方向解耦，消除循环依赖。

### 领域模型

| 概念 | 包 | 角色 | 关键字段 / 方法 |
|---|---|---|---|
| `Agent` | `internal/agent` | AI 角色的身份与运行配置（一对多会话） | `id / name / system_prompt`、`tools []string`（工具白名单）、`temperature`、`max_iterations` |
| `Session` | `internal/session` | 一次对话的上下文容器，保存完整消息历史 | `session_id / agent_id`、`messages []message.Message` |
| `Message` | `internal/message` | 对话中的一条原子记录，三段式会话的骨架 | `role`（`user/assistant/tool`）、`content`、`tool_calls`、`tool_call_id` |
| `Task` | `internal/task` | 一次运行请求 + 状态机；同一 Session 可反复提交多个 Task | `agent_id / session_id / input`、`status`、`output / error`、`created_at / started_at / ended_at` |
| `model.Client` | `internal/model` | 大模型调用抽象（接口），引擎只认它不认识网关 | `Chat(ctx, Request) (*Response, error)`；实现：`GatewayClient` |
| `tool.Tool` | `internal/tool` | Agent 可执行能力的抽象（接口） | `Name / Description / InputSchema / Execute(ctx, args)` |
| `memory.Memory` | `internal/memory` | 会话消息的可插拔持久化后端（接口，可选注入） | `Get(ctx, sessionID)`、`Save(ctx, sessionID, msg)`；实现：`InMemory`、`FileStore` |
| `AgentContext` | `internal/agentcontext` | 每次模型调用的上下文组装（system prompt + 历史 + 工具契约） | `Build(...)` → `ToModelMessages()` |

模型之间有一条重要的数量关系：**Agent 与 Session 一对多**（一个角色可服务多场对话），**Session 与 Task 一对多**（一场对话可发起多次可追踪的运行）。

### 组合根（main.go）

`cmd/agent-runtime/main.go` 按固定顺序完成装配：

1. `config.Load("configs/config.json")` 加载配置；
2. `new` 出 4 个容器：`agent.Registry`、`session.Manager`、`tool.Registry`、`task.Manager`；
3. 造模型客户端：`gateway.base_url` 非空 → `model.NewGatewayClient(baseURL, WithTimeout, WithRetries)`；为空 → 演示模型 `demoModel`；
4. 注入组装：
   - `runtime.New(agents, sessions, modelClient, tools, taskManager)` → 引擎
   - `runtime.NewExecutor(taskManager, rt)` → 执行器（实现 `task.Executor`）
   - `task.NewServiceWithPool(taskManager, executor, workers, queueSize)` → 任务服务
   - `rt.SetMemory(mem)` → 可选注入消息持久化
   - `transport.NewServer(taskService, sessions).Handler()` → HTTP 处理器
5. 注册演示工具 `get_time` 与演示 Agent `demo-agent`；
6. 若为演示模式（无网关），启动前先同步跑一次闭环验证；
7. 启动 HTTP 服务，监听 `SIGINT/SIGTERM` 优雅退出（停收连接 → 关队列 → 等 Worker 排空 → 落盘任务）。

### 一次任务的运行时数据流

```
HTTP POST /v1/tasks
   │
   ▼
task.Service.Submit ──校验──▶ 创建 Task(pending) ──▶ 写入有界队列(容量可配)
   │（立即返回 202，不阻塞调用方）                          │
                                                          ▼
                                          空闲 Worker 取出任务
                                                          │
                                                          ▼
                              runtime.Executor.Execute ──▶ Task: pending→running
                                                          │
                                                          ▼
                                       runtime.Runtime.Run（Agent Loop）
                                         1. 用户输入写为 user 消息
                                         2. 循环：检查 ctx → 读取会话历史
                                            → agentcontext.Build → model.Chat
                                         3. 无 tool_calls：存 assistant 消息，返回
                                         4. 有 tool_calls：存消息 → 逐个执行工具
                                            → 结果以 role=tool 回写 → 回到 2
                                                          │
                                          ┌───────────────┼────────────────┐
                                          ▼               ▼                ▼
                                    成功 → completed   模型/迭代错误→failed   ctx 取消→canceled
                                                          │
                                                          ▼
                                    任务/消息写盘（tasks.json / messages.json），重启可恢复
```

### 关键设计决策

1. **状态机是唯一的裁判**：HTTP 层、执行器、取消请求都只操作 `Task` 状态机；非法流转返回错误，取消非 `running` 任务返回 `409`，并发下状态不会乱。
2. **先内存、后落盘、失败不阻塞**：会话历史是模型下一轮推理的依据，必须写成功；`memory.Memory` 只是可选「影子持久化」，写失败最多丢失恢复能力，绝不打断主流程。
3. **取消是 context 全链路传导**：HTTP 取消 → `running` 表中找到取消函数 → 取消独立 context → Agent Loop 每轮开头的 `ctx.Done()` 立即生效。
4. **深拷贝防数据竞争**：Registry / Manager 的 `Get / List` 返回副本而非内部指针（已过 `-race`）。
5. **异步任务用独立生命周期**：Worker 执行任务使用 `context.Background()` 派生的独立 context，父级（HTTP 请求）结束不影响后台任务继续运行。

---

## 目录结构

```
.
├── cmd/agent-runtime/          # 组合根（唯一装配点）
│   ├── main.go                 #   装配 + 启动 + 优雅退出
│   └── demo.go                 #   演示模型 demoModel + 工具 timeTool
├── configs/
│   └── config.json             # 运行配置（端口 / 网关 / Worker / 数据目录）
├── internal/
│   ├── agent/                  # 领域：Agent 定义 + 注册表
│   ├── agentcontext/           # 领域：模型调用上下文组装
│   ├── config/                 # 配置加载（默认值回退）
│   ├── memory/                 # 记忆接口 + InMemory + JSON FileStore
│   ├── message/                # 领域：消息模型（role / tool_calls）
│   ├── model/                  # 模型抽象 + GatewayClient（超时/重试/工具下发）
│   ├── runtime/                # 编排层：Agent Loop + Executor
│   ├── session/                # 领域：会话管理（消息历史）
│   ├── task/                   # 任务层：状态机 / Worker 池 / 有界队列 / 存储
│   ├── tool/                   # 工具接口 + 注册表
│   └── transport/              # HTTP 传输层（示例）
├── data/                       # 运行期生成：tasks.json / messages.json
├── go.mod                      # module github.com/Jeffery-source/agent-runtime (go 1.26.1)
└── tests/                      # （预留）端到端测试目录
```

每个 `internal/*` 包内的 `*_test.go` 为同包单元测试（详见[测试](#测试)）。

---

## 快速开始

### 环境要求

- Go ≥ 1.26（`go.mod` 声明 `go 1.26.1`）
- 无需真实 AI 网关——内置演示模型即可跑通完整闭环

### 启动

```bash
cd /path/to/agent_runtime          # 必须在此目录运行（配置为相对路径）
go run ./cmd/agent-runtime
```

日志预期：

```
Agent Runtime starting
using demo model (set gateway.base_url to use a real gateway)
sync run: status=completed content="演示闭环完成：已通过 get_time 工具获取时间。"
HTTP server listening on :8080
```

演示模式下启动时会**自动同步跑一次闭环**：创建 `demo-session`，让 `demo-agent` 回答"现在几点"，其中模型奇数轮请求调用 `get_time` 工具、拿到结果后偶数轮输出最终答案——以此证明「输入 → 模型 → 工具 → 输出」链路可用。

启动后工程根目录下会生成 `data/tasks.json` 与 `data/messages.json`（消息在 `data_dir` 配置下）。

### 端到端演示（HTTP）

```bash
# 1. 创建会话（省略 session_id 时自动生成 UUID）
curl -s -X POST localhost:8080/v1/sessions \
  -H 'Content-Type: application/json' \
  -d '{"agent_id":"demo-agent","user_id":"alice"}'

# 2. 提交任务（注意：必须携带 agent_id）
curl -s -X POST localhost:8080/v1/tasks \
  -H 'Content-Type: application/json' \
  -d '{"agent_id":"demo-agent","session_id":"<上一步返回的 session_id>","input":"现在几点了"}'

# 3. 轮询任务结果（异步执行，观察 pending → completed）
curl -s localhost:8080/v1/tasks/<task_id>

# 4. 取消任务（仅对运行中的任务生效）
curl -s -X POST localhost:8080/v1/tasks/<task_id>/cancel
```

完整链路已在开发中实测通过（`go test -race ./...` 全绿 + HTTP 端到端验证）。

---

## HTTP API

基础地址：`http://<host>:8080`（端口见配置 `server.addr`）。

| 方法 | 路径 | 说明 | 成功状态码 |
|---|---|---|---|
| POST | `/v1/sessions` | 创建会话 | `201` |
| POST | `/v1/tasks` | 提交任务（异步执行，立即返回） | `202` |
| GET | `/v1/tasks/{id}` | 查询任务状态与结果 | `200` |
| POST | `/v1/tasks/{id}/cancel` | 取消正在运行的任务 | `200` |

### 创建会话

请求：

```json
{
  "agent_id": "demo-agent",      // 必填
  "user_id": "alice",            // 可选，业务标识
  "session_id": ""               // 可选，省略则服务端生成 UUID
}
```

响应 `201`：

```json
{ "session_id": "f47ac10b-…", "agent_id": "demo-agent" }
```

错误：`400`（请求体非法 / 缺少 `agent_id`）、`409`（会话已存在）。

### 提交任务

请求：

```json
{
  "agent_id": "demo-agent",      // 必填
  "session_id": "f47ac10b-…",    // 必填
  "input": "现在几点了"           // 必填
}
```

响应 `202`（任务已入队，尚未执行）：

```json
{
  "id": "5f7c1a2e-…",
  "agent_id": "demo-agent",
  "session_id": "f47ac10b-…",
  "input": "现在几点了",
  "status": "pending",
  "created_at": "2026-09-01T10:00:00+08:00"
}
```

错误：`400`（字段缺失或为空）、`503`（任务队列已满，`ErrQueueFull` 背压——可稍后重试）。

### 查询任务

响应 `200`：

```json
{
  "id": "5f7c1a2e-…",
  "agent_id": "demo-agent",
  "session_id": "f47ac10b-…",
  "input": "现在几点了",
  "status": "completed",
  "output": "演示闭环完成：已通过 get_time 工具获取时间。",
  "created_at": "2026-09-01T10:00:00+08:00",
  "started_at": "2026-09-01T10:00:00.5+08:00",
  "ended_at": "2026-09-01T10:00:01+08:00"
}
```

> `status` 取值：`pending` / `running` / `completed` / `failed` / `canceled`。
> 失败时字段 `error` 携带原因；`output` / `error` / `started_at` / `ended_at` 仅在相应阶段出现。

错误：`404`（任务不存在）。

### 取消任务

响应 `200`：

```json
{ "status": "canceled" }
```

错误：`404`（任务不存在）、`409`（任务不处于 `running`，无法取消，响应体含原因）。

---

## 配置说明

默认配置 `configs/config.json`：

```json
{
  "server": { "addr": ":8080" },
  "gateway": {
    "base_url": "",
    "timeout": "30s",
    "retries": 1
  },
  "workers": 4,
  "queue_size": 128,
  "data_dir": "./data"
}
```

| 字段 | 含义 | 缺省行为 |
|---|---|---|
| `server.addr` | HTTP 监听地址 | 默认 `:8080` |
| `gateway.base_url` | AI 网关地址；**为空则使用演示模型** | 空 |
| `gateway.timeout` | 单次模型请求超时（`GatewayClient`） | 非法/缺失回退 `30s` |
| `gateway.retries` | 模型请求重试次数 | 默认 1 |
| `workers` | Worker 池大小（并行执行的任务数上限） | `<=0` 回退 4 |
| `queue_size` | 有界队列容量（排队上限，超出返回 503） | `<=0` 回退 128 |
| `data_dir` | 数据目录（`tasks.json` 任务、`messages.json` 会话消息） | 为空则不启用文件持久化 |

---

## 扩展指南

### 新增一个 Agent

在组合根中定义并注册（参考 `main.go` 的 `demoAgent`）：

```go
agents.Register(&agent.Agent{
    ID:            "support-agent",
    Name:          "Support Agent",
    Description:   "IT 支持助手",
    Model:         "your-model-name",      // 由接入的网关解释
    SystemPrompt:  "You are a helpful IT support assistant.",
    Tools:         []string{"get_time", "your_tool_name"}, // 工具白名单
    MaxIterations: 5,                                      // 缺省 10
})
```

> `Agent.Tools` 里声明的工具名，引擎会到 `tool.Registry` 解析成工具契约下发给模型；未注册的工具名会被静默忽略。

### 实现并注册一个工具

工具只需实现 4 个方法（接口定义在 `internal/tool/model.go`）：

```go
type weatherTool struct{}

func (t *weatherTool) Name() string        { return "get_weather" }
func (t *weatherTool) Description() string { return "查询指定城市的天气" }

// InputSchema 是对模型声明的 JSON Schema，[]byte。
func (t *weatherTool) InputSchema() []byte {
    return []byte(`{
        "type": "object",
        "properties": {
            "city": {"type": "string", "description": "城市名"}
        },
        "required": ["city"],
        "additionalProperties": false
    }`)
}

// Execute 参数是模型按 Schema 生成的 JSON 参数。
func (t *weatherTool) Execute(ctx context.Context, arguments []byte) (string, error) {
    // 解析 arguments → 调用业务能力 → 返回文本结果（会以 role=tool 消息回喂模型）
    return "晴，26℃", nil
}
```

然后在组合根注册，并把名字加进 Agent 白名单：

```go
tools.Register(&weatherTool{})                       // 注册到工具注册表
// Agent.Tools = []string{"get_weather", ...}        // 声明可用
```

> 工具的业务错误不视为任务失败：`Execute` 返回 error 时，错误文本会作为工具结果回喂模型，让模型自行决定下一步；只有模型调用层面的错误才使任务进入 `failed`。

### 接入真实模型网关

把配置中的 `gateway.base_url` 填为网关地址即可，其余代码零改动：

```json
{
  "gateway": {
    "base_url": "https://your-ai-gateway.example.com",
    "timeout": "60s",
    "retries": 2
  }
}
```

启动时 `newModelClient` 会据此构造 `GatewayClient`（自动启用：请求超时、失败重试、工具定义序列化下发）。`GatewayClient` 还支持通过 `model.WithHTTPClient` 注入自定义 `http.Client`（用于自定义鉴权头、TLS 等）。

### 更换持久化后端

会话消息持久化实现 `memory.Memory` 接口即可，例如换成 Redis / 数据库：

```go
type Memory interface {
    Get(ctx context.Context, sessionID string) ([]message.Message, error)
    Save(ctx context.Context, sessionID string, message message.Message) error
}
```

在组合根通过 `rt.SetMemory(yourImpl)` 注入。**不注入则纯内存运行**。任务存储同理：`task.FileStore` 实现了 `Save / Load`，可替换为任意后端（参照 `internal/task/store.go`）。

---

## 测试

```bash
go build ./...       # 编译检查
go vet ./...         # 静态检查
go test -race ./...  # 全量测试（含竞态检测）
```

覆盖情况：

- 领域层：`agent`（注册表深拷贝）、`session`（管理）、`task`（状态机 / Worker 池 / 有界队列 / 存储恢复）
- 编排层：`runtime`（Agent Loop / Executor / 错误哨兵 / 集成）
- 支撑层：`config`（默认值回退）、`memory`（文件持久化往返）、`transport`（HTTP 错误映射，含 503 / 404 / 409 语义）、`model`（网关客户端）

`go test -race ./...` 全绿。

---

## 常见问题

**Q：为什么 `POST /v1/tasks` 不带 `agent_id` 会报错？**
传输层不负责从会话反查 Agent，任务必须显式声明由哪个 Agent 执行（`agent_id` 与会话所属 Agent 一致性由上层业务保证）。

**Q：任务提交后没立刻执行完？**
`Submit` 是异步的：任务先进入有界队列，由 Worker 池消费。用 `GET /v1/tasks/{id}` 轮询状态直到 `completed` / `failed` / `canceled`。

**Q：并发提交很多任务会怎样？**
最多 `workers` 个任务并行执行，其余在队列排队（默认 128）；队列满时 `Submit` 返回 503（`ErrQueueFull`），调用方可稍后重试——这是刻意的背压设计，避免无界排队拖垮进程。

**Q：进程重启后数据还在吗？**
在，前提是配置了 `data_dir`：任务写入 `tasks.json`、会话消息写入 `messages.json`，启动时自动恢复。

**Q：取消一个已完成的任务？**
不允许。取消只对 `running` 任务生效，其余返回 `409 not running`——这是状态机守卫的预期行为。

**Q：如何让模型看到工具定义？**
Agent 白名单 → 引擎解析为 `model.ToolDefinition`（名称/描述/JSON Schema）→ 随 `model.Request.Tools` 下发给 `model.Client`。只要网关实现了工具下发协议即可。

---

## 许可证 / 说明

演示模型与演示工具仅用于在无真实网关时验证闭环，生产环境请通过 `configs/config.json` 接入真实 AI 网关，并为 Agent / 工具补充业务侧实现与鉴权。
