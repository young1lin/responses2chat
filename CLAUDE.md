# responses2chat - Claude Code 项目指南

## ⚠️ 安全警告

**绝对禁止提交任何 API Key、密钥或敏感信息到代码库！**

- 所有包含敏感信息的配置文件已在 `.gitignore` 中排除
- 测试脚本 (`test_*.sh`) 默认被忽略，因为可能包含硬编码的 API Key
- **提交前必须检查**: `git diff --staged` 确认没有敏感信息
- 如果不小心提交了 API Key，**立即更换该 Key**

## 项目概述

一个将 OpenAI **Responses API** 请求转换为 **Chat Completions API** 格式的代理服务器，让 [Codex CLI](https://github.com/openai/codex) 能够与第三方 LLM 提供商一起工作。

## 技术栈

- **语言**: Go 1.25+
- **存储**: BBolt (嵌入式 KV 存储)
- **日志**: Uber Zap
- **配置**: Viper + mapstructure

## 项目结构

```
responses2chat/
├── cmd/responses2chat/      # 主入口
├── internal/
│   ├── config/              # 配置管理
│   ├── converter/           # API 格式转换
│   ├── handler/             # HTTP 处理器
│   ├── models/              # 数据模型
│   └── storage/             # BBolt 存储
├── pkg/logger/              # 日志工具
├── configs/                 # 配置文件
├── codex/                   # Codex CLI 源码 (submodule)
└── CLAUDE.md               # 本文件
```

## Codex CLI 子模块

项目包含 OpenAI Codex CLI 的源码作为 git submodule，方便排查兼容性问题。

### 关键目录

| 目录 | 说明 |
|------|------|
| `codex/codex-rs/` | Rust 源码主目录 |
| `codex/codex-rs/cli/` | CLI 入口 |
| `codex/codex-rs/core/` | 核心逻辑 |
| `codex/codex-rs/config/` | 配置处理 |
| `codex/codex-rs/codex-api/` | API 客户端 |
| `codex/codex-rs/protocol/` | 协议定义 |
| `codex/codex-rs/responses-api-proxy/` | Responses API 代理（重要！）|
| `codex/codex-rs/tui/` | 终端 UI |

### 排查 Codex 问题时

1. **API 兼容性问题**: 查看 `codex/codex-rs/codex-api/` 和 `codex/codex-rs/protocol/`
2. **配置问题**: 查看 `codex/codex-rs/config/`
3. **Responses API 调用**: 查看 `codex/codex-rs/responses-api-proxy/`
4. **SSE 事件处理**: 查看 `codex/codex-rs/core/` 中的流式响应处理

### 更新 Submodule

```bash
git submodule update --remote codex
```

## 已实现功能 (基于 OpenAI 迁移文档)

| 功能 | 状态 |
|------|------|
| `POST /v1/responses` | ✅ |
| `GET /v1/responses/{id}` | ✅ |
| `previous_response_id` 多轮对话 | ✅ |
| 流式响应 (SSE) | ✅ |
| 流式响应历史存储 | ✅ |
| BBolt 持久化 | ✅ |
| 工具/函数调用 | ✅ |

## 运行测试

```bash
go test ./... -v
```

## 配置 Codex 使用本代理

**~/.codex/config.toml**:
```toml
model_provider = "custom"
model = "deepseek-chat"

[model_providers.custom]
name = "Responses2Chat"
base_url = "http://127.0.0.1:8080/v1"
wire_api = "responses"
env_key = "API_KEY"
```

## 相关文档

- [OpenAI 迁移文档](https://developers.openai.com/api/docs/guides/migrate-to-responses)
- [COMPATIBILITY.md](COMPATIBILITY.md) - 兼容性检查清单
- [codex/AGENTS.md](codex/AGENTS.md) - Codex 官方 Agent 文档
