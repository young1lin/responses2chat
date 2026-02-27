# responses2chat

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/young1lin/responses2chat)](https://github.com/young1lin/responses2chat/releases)

[English](README_EN.md) | 简体中文

一个将 OpenAI **Responses API** 请求转换为 **Chat Completions API** 格式的代理服务器，让 [Codex CLI](https://github.com/openai/codex) 能够与第三方 LLM 提供商一起工作。

## 为什么需要这个项目？

OpenAI Codex 现在只支持 Responses API (`/v1/responses`)，但大多数第三方提供商（DeepSeek、智谱 AI、通义千问、LongCat、StepFun 等）只支持 Chat Completions API (`/v1/chat/completions`)。这个代理填补了这一空白。

## 功能特性

- 🔄 完整的 API 格式转换（Responses → Chat Completions）
- 🌊 流式响应支持（SSE）
- 🔧 多提供商配置
- 🔑 API Key 透传或环境变量配置
- 📊 结构化日志（Uber Zap + TraceID 追踪）
- 🛠️ 工具/函数调用支持
- 🔍 请求追踪（X-Trace-ID 支持）
- 💾 **多轮对话支持**（`previous_response_id` + BBolt 持久化存储）
- 📜 **历史查询接口**（`GET /v1/responses/{id}`）

## 更新日志

### v0.0.2 (2026-02-27)

**新功能：多轮对话支持**

基于 [OpenAI 官方迁移文档](https://developers.openai.com/api/docs/guides/migrate-to-responses) 实现：

- ✅ `previous_response_id` 多轮对话 - 自动拼接历史消息
- ✅ `GET /v1/responses/{id}` 历史查询接口
- ✅ BBolt 持久化存储 - 重启不丢失对话
- ✅ 流式响应历史存储

**技术实现：**

| 功能 | 说明 |
|------|------|
| 存储层 | BBolt (纯 Go 嵌入式 KV) |
| 并发安全 | MVCC |
| 默认路径 | `./data/conversations.db` |

**文件变更：**

```
新增:
  internal/storage/storage.go          # BBolt 存储层
  internal/storage/storage_test.go     # 存储测试
  internal/converter/converter_test.go # 转换测试
  COMPATIBILITY.md                      # 兼容性文档

修改:
  internal/handler/handler.go          # 集成存储 + GET 接口
  internal/converter/converter.go      # 支持 history 参数
  internal/converter/streaming.go      # 流式响应存储
```

## 安装

### 从源码构建
```bash
git clone https://github.com/young1lin/responses2chat.git
cd responses2chat
make build
```

### 使用 Go Install
```bash
go install github.com/young1lin/responses2chat/cmd/responses2chat@latest
```

### 下载二进制文件
从 [GitHub Releases](https://github.com/young1lin/responses2chat/releases) 下载

## 快速开始

1. **创建配置文件** (`config.yaml`):
```yaml
server:
  port: 8080

default_target:
  base_url: "https://api.deepseek.com"
  path_suffix: "/v1/chat/completions"

logging:
  level: "info"
```

2. **启动代理服务器**:
```bash
# 通过环境变量设置 API Key (Unix/Linux/macOS)
export R2C_DEFAULT_API_KEY="your-api-key"

# 或通过命令行启动
./bin/responses2chat -c config.yaml
```

```powershell
# PowerShell (Windows)
$env:R2C_DEFAULT_API_KEY="your-api-key"

# 或通过命令行启动
.\bin\responses2chat.exe -c config.yaml
```

3. **配合 Codex 使用**:
```bash
# 创建 Codex 配置 (~/.codex/config.toml)
codex -c 'model_providers.proxy = { name = "Proxy", base_url = "http://127.0.0.1:8080/v1", wire_api = "responses" }' \
      -c 'model_provider = "proxy"' \
      "你好，世界！"
```

## 配置

### 命令行参数

| 参数 | 描述 | 默认值 |
|------|------|--------|
| `-c, --config` | 配置文件路径 | 自动查找 |
| `-p, --port` | 监听端口 | `8080` |
| `-v, --version` | 显示版本 | |

### 配置文件查找优先级

不指定 `-c` 参数时，按以下顺序查找 `config.yaml`：

1. 当前目录 (`./config.yaml`) — **优先**
2. `./configs/config.yaml`
3. `../configs/config.yaml` (从 `bin/` 目录运行时)

### 环境变量

| 变量 | 描述 |
|------|------|
| `R2C_PORT` | 监听端口 |
| `R2C_CONFIG` | 配置文件路径 |
| `R2C_LOG_LEVEL` | 日志级别 (debug/info/warn/error) |
| `R2C_DEFAULT_API_KEY` | 默认 API Key |
| `R2C_PROVIDER_<NAME>_API_KEY` | 特定提供商的 API Key |

### 多提供商配置

```yaml
providers:
  deepseek:
    base_url: "https://api.deepseek.com"
    path_suffix: "/v1/chat/completions"

  longcat:
    base_url: "https://api.longcat.chat/openai"
    path_suffix: "/v1/chat/completions"

  stepfun:
    base_url: "https://api.stepfun.com/v1"
    path_suffix: "/chat/completions"

  zhipu:
    base_url: "https://open.bigmodel.cn/api/coding/paas/v4"
    path_suffix: "/chat/completions"

  ollama:
    base_url: "http://localhost:11434"
    path_suffix: "/v1/chat/completions"
```

使用不同的提供商：
```bash
# 通过 URL 路径
curl http://localhost:8080/deepseek/v1/responses
curl http://localhost:8080/longcat/v1/responses

# 通过 Header
curl -H "X-Target-Provider: deepseek" http://localhost:8080/v1/responses
```

## API 端点

| 端点 | 描述 |
|------|------|
| `POST /v1/responses` | 默认提供商 |
| `POST /{provider}/v1/responses` | 指定提供商 |
| `GET /v1/responses/{id}` | 查询对话历史 |
| `GET /health` | 健康检查 |
| `GET /providers` | 列出可用提供商 |

## Codex CLI 配置

### 方法一：单个提供商

**~/.codex/config.toml**:
```toml
model = "deepseek-chat"
model_provider = "proxy"

[model_providers.proxy]
name = "Proxy"
base_url = "http://127.0.0.1:8080/v1"
wire_api = "responses"
env_key = "DEEPSEEK_API_KEY"
```

```bash
export DEEPSEEK_API_KEY="your-api-key"
codex "你好"
```

### 方法二：多 Profile 配置（推荐）

**~/.codex/config.toml**:
```toml
# 默认 profile
model = "LongCat-Flash-Lite"
model_provider = "longcat"

# LongCat profile（免费额度）
[profiles.longcat]
model = "LongCat-Flash-Lite"
model_provider = "longcat"

[profiles.longcat.model_providers.longcat]
name = "LongCat"
base_url = "http://127.0.0.1:8080/longcat/v1"
wire_api = "responses"
env_key = "LONGCAT_API_KEY"

# DeepSeek profile
[profiles.deepseek]
model = "deepseek-chat"
model_provider = "deepseek"

[profiles.deepseek.model_providers.deepseek]
name = "DeepSeek"
base_url = "http://127.0.0.1:8080/deepseek/v1"
wire_api = "responses"
env_key = "DEEPSEEK_API_KEY"

# StepFun profile
[profiles.stepfun]
model = "step-3.5-flash"
model_provider = "stepfun"

[profiles.stepfun.model_providers.stepfun]
name = "StepFun"
base_url = "http://127.0.0.1:8080/stepfun/v1"
wire_api = "responses"
env_key = "STEPFUN_API_KEY"
```

**使用方式：**
```bash
# 终端1 - LongCat（免费）
export LONGCAT_API_KEY="your-longcat-key"
codex --profile longcat

# 终端2 - DeepSeek
export DEEPSEEK_API_KEY="your-deepseek-key"
codex --profile deepseek

# 终端3 - StepFun
export STEPFUN_API_KEY="your-stepfun-key"
codex --profile stepfun
```

```powershell
# PowerShell (Windows)
$env:LONGCAT_API_KEY="your-longcat-key"
codex --profile longcat

$env:DEEPSEEK_API_KEY="your-deepseek-key"
codex --profile deepseek

$env:STEPFUN_API_KEY="your-stepfun-key"
codex --profile stepfun
```

## 支持的提供商

| 提供商 | Base URL | 模型 |
|--------|----------|------|
| DeepSeek | `https://api.deepseek.com` | deepseek-chat, deepseek-reasoner |
| LongCat | `https://api.longcat.chat/openai` | LongCat-Flash-Lite, LongCat-Flash-Thinking |
| StepFun 阶跃星辰 | `https://api.stepfun.com/v1` | step-3.5-flash |
| 智谱 AI | `https://open.bigmodel.cn/api/coding/paas/v4` | glm-5 |
| 通义千问 | `https://dashscope.aliyuncs.com/compatible-mode/v1` | qwen-turbo |
| Ollama | `http://localhost:11434` | llama3, 等 |
| LMStudio | `http://localhost:1234` | 本地模型 |

## 请求追踪

每个请求都会分配一个 TraceID 用于调试：

- **输入**：检查 `X-Trace-ID`、`X-Request-ID`、`X-Correlation-ID` 等 Header
- **输出**：在响应中返回 `X-Trace-ID` Header
- **日志**：所有日志都包含 TraceID

日志示例：
```
INFO    request received        {"trace_id": "abc123def456", "method": "POST", "path": "/v1/responses"}
INFO    sending request to target {"trace_id": "abc123def456", "provider": "deepseek", "target_url": "..."}
INFO    request completed       {"trace_id": "abc123def456", "duration_ms": 1523}
```

## API 转换参考

### 请求转换

| Responses API | Chat Completions |
|---------------|------------------|
| `instructions` | `messages[role="system"]` |
| `input[]` | `messages[]` |
| `function_call` | `tool_calls` |
| `function_call_output` | `messages[role="tool"]` |
| `role: developer` | `role: system` |

### 响应转换

| Chat Completions | Responses API |
|------------------|---------------|
| `choices[0].message` | `output[output_item]` |
| `delta.content` (SSE) | `response.output_text.delta` |
| `delta.tool_calls` | `response.output_item.added` |

## Docker

```bash
# 构建
docker build -t responses2chat .

# 运行
docker run -p 8080:8080 \
  -e R2C_DEFAULT_API_KEY=your-api-key \
  responses2chat
```

## 开发

### 构建
```bash
make build
```

### 运行
```bash
make run
```

### 测试
```bash
# 健康检查
curl http://localhost:8080/health

# 非流式请求
curl -X POST http://localhost:8080/v1/responses \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-chat",
    "instructions": "You are helpful.",
    "input": [{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "Hello"}]}],
    "stream": false
  }'

# 流式请求
curl -X POST http://localhost:8080/v1/responses \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "deepseek-chat",
    "instructions": "You are helpful.",
    "input": [{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "Hello"}]}],
    "stream": true
  }'
```

## 许可证

[MIT License](LICENSE)
