# responses2chat

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/young1lin/responses2chat)](https://github.com/young1lin/responses2chat/releases)

English | [简体中文](README.md)

A proxy server that converts OpenAI **Responses API** requests to **Chat Completions API** format, enabling [Codex CLI](https://github.com/openai/codex) to work with third-party LLM providers.

## Why?

OpenAI Codex now exclusively uses the Responses API (`/v1/responses`), but most third-party providers (DeepSeek, Zhipu AI, Qwen, LongCat, StepFun, etc.) only support the Chat Completions API (`/v1/chat/completions`). This proxy bridges that gap.

## Features

- 🔄 Full API format conversion (Responses → Chat Completions)
- 🌊 Streaming response support (SSE)
- 🔧 Multi-provider configuration
- 🔑 API key passthrough or environment variable configuration
- 📊 Structured logging with Uber Zap + Trace ID tracking
- 🛠️ Tool/function call support
- 🔍 Request tracing with X-Trace-ID support

## Installation

### From Source
```bash
git clone https://github.com/young1lin/responses2chat.git
cd responses2chat
make build
```

### Using Go Install
```bash
go install github.com/young1lin/responses2chat/cmd/responses2chat@latest
```

### Download Binary
Download from [GitHub Releases](https://github.com/young1lin/responses2chat/releases)

## Quick Start

1. **Create config file** (`config.yaml`):
```yaml
server:
  port: 8080

default_target:
  base_url: "https://api.deepseek.com"
  path_suffix: "/v1/chat/completions"

logging:
  level: "info"
```

2. **Start the proxy**:
```bash
# Set API key via environment variable (Unix/Linux/macOS)
export R2C_DEFAULT_API_KEY="your-api-key"

# Or pass via command line
./bin/responses2chat -c config.yaml
```

```powershell
# PowerShell (Windows)
$env:R2C_DEFAULT_API_KEY="your-api-key"

# Or pass via command line
.\bin\responses2chat.exe -c config.yaml
```

3. **Use with Codex**:
```bash
codex -c 'model_providers.proxy = { name = "Proxy", base_url = "http://127.0.0.1:8080/v1", wire_api = "responses" }' \
      -c 'model_provider = "proxy"' \
      "Hello, world!"
```

## Configuration

### Command Line Flags

| Flag | Description | Default |
|------|-------------|---------|
| `-c, --config` | Config file path | `./config.yaml` |
| `-p, --port` | Listen port | `8080` |
| `-v, --version` | Show version | |

### Environment Variables

| Variable | Description |
|----------|-------------|
| `R2C_PORT` | Listen port |
| `R2C_CONFIG` | Config file path |
| `R2C_LOG_LEVEL` | Log level (debug/info/warn/error) |
| `R2C_DEFAULT_API_KEY` | Default API key |
| `R2C_PROVIDER_<NAME>_API_KEY` | Provider-specific API key |

### Multi-Provider Setup

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

Use different providers:
```bash
# Via URL path
curl http://localhost:8080/deepseek/v1/responses
curl http://localhost:8080/longcat/v1/responses

# Via header
curl -H "X-Target-Provider: deepseek" http://localhost:8080/v1/responses
```

## API Endpoints

| Endpoint | Description |
|----------|-------------|
| `POST /v1/responses` | Default provider |
| `POST /{provider}/v1/responses` | Specified provider |
| `GET /health` | Health check |
| `GET /providers` | List available providers |

## Codex CLI Configuration

### Method 1: Single Provider

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
codex "Hello"
```

```powershell
# PowerShell (Windows)
$env:DEEPSEEK_API_KEY="your-api-key"
codex "Hello"
```

### Method 2: Multiple Profiles (Recommended)

**~/.codex/config.toml**:
```toml
# Default profile
model = "LongCat-Flash-Lite"
model_provider = "longcat"

# LongCat profile (free tier)
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
```

**Usage:**
```bash
# Terminal 1 - LongCat (free)
export LONGCAT_API_KEY="your-longcat-key"
codex --profile longcat

# Terminal 2 - DeepSeek
export DEEPSEEK_API_KEY="your-deepseek-key"
codex --profile deepseek
```

```powershell
# PowerShell (Windows)
$env:LONGCAT_API_KEY="your-longcat-key"
codex --profile longcat

$env:DEEPSEEK_API_KEY="your-deepseek-key"
codex --profile deepseek
```

## Supported Providers

| Provider | Base URL | Models |
|----------|----------|--------|
| DeepSeek | `https://api.deepseek.com` | deepseek-chat, deepseek-reasoner |
| LongCat | `https://api.longcat.chat/openai` | LongCat-Flash-Lite, LongCat-Flash-Thinking |
| StepFun | `https://api.stepfun.com/v1` | step-3.5-flash |
| Zhipu AI | `https://open.bigmodel.cn/api/coding/paas/v4` | glm-5 |
| Qwen | `https://dashscope.aliyuncs.com/compatible-mode/v1` | qwen-turbo |
| Ollama | `http://localhost:11434` | llama3, etc. |
| LMStudio | `http://localhost:1234` | Local models |

## Request Tracing

Every request is assigned a trace ID for easy debugging:

- **Input**: Checks `X-Trace-ID`, `X-Request-ID`, `X-Correlation-ID` headers
- **Output**: Returns `X-Trace-ID` header in response
- **Logs**: All logs include the trace ID

## API Conversion Reference

### Request Conversion

| Responses API | Chat Completions |
|---------------|------------------|
| `instructions` | `messages[role="system"]` |
| `input[]` | `messages[]` |
| `function_call` | `tool_calls` |
| `function_call_output` | `messages[role="tool"]` |
| `role: developer` | `role: system` |

### Response Conversion

| Chat Completions | Responses API |
|------------------|---------------|
| `choices[0].message` | `output[output_item]` |
| `delta.content` (SSE) | `response.output_text.delta` |
| `delta.tool_calls` | `response.output_item.added` |

## Docker

```bash
# Build
docker build -t responses2chat .

# Run
docker run -p 8080:8080 \
  -e R2C_DEFAULT_API_KEY=your-api-key \
  responses2chat
```

## Development

### Build
```bash
make build
```

### Run
```bash
make run
```

### Test
```bash
# Health check
curl http://localhost:8080/health

# Non-streaming request
curl -X POST http://localhost:8080/v1/responses \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"model": "deepseek-chat", "instructions": "You are helpful.", "input": [{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "Hello"}]}], "stream": false}'

# Streaming request
curl -X POST http://localhost:8080/v1/responses \
  -H "Authorization: Bearer your-api-key" \
  -H "Content-Type: application/json" \
  -d '{"model": "deepseek-chat", "instructions": "You are helpful.", "input": [{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "Hello"}]}], "stream": true}'
```

## License

[MIT License](LICENSE)
