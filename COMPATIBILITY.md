# Responses API 兼容性检查清单

基于 [OpenAI 官方迁移文档](https://developers.openai.com/api/docs/guides/migrate-to-responses)

## 核心功能

| 功能 | 状态 | 实现位置 | 测试覆盖 |
|------|------|---------|---------|
| `POST /v1/responses` | ✅ | `internal/handler/handler.go` | - |
| `GET /v1/responses/{id}` | ✅ | `internal/handler/handler.go` | - |
| `previous_response_id` 多轮对话 | ✅ | `internal/handler/handler.go:246-257` | ✅ |
| 流式响应 (`stream: true`) | ✅ | `internal/converter/streaming.go` | - |
| 流式响应历史存储 | ✅ | `internal/handler/handler.go:359-410` | - |

## 请求转换 (Responses → Chat Completions)

| 功能 | 状态 | 实现位置 | 测试覆盖 |
|------|------|---------|---------|
| `input` → `messages` | ✅ | `internal/converter/converter.go` | ✅ |
| `instructions` → system message | ✅ | `internal/converter/converter.go:30-40` | ✅ |
| `developer` role → `system` | ✅ | `internal/converter/converter.go:86` | ✅ |
| `function_call` input | ✅ | `internal/converter/converter.go:126-143` | ✅ |
| `function_call_output` input | ✅ | `internal/converter/converter.go:146-152` | ✅ |
| `tools` (function type only) | ✅ | `internal/converter/converter.go:53-63` | ✅ |
| `temperature` | ✅ | `internal/converter/converter.go:66-68` | - |
| `max_output_tokens` → `max_tokens` | ✅ | `internal/converter/converter.go:69-71` | - |
| 历史消息拼接 | ✅ | `internal/converter/converter.go:22-29` | ✅ |

## 响应转换 (Chat Completions → Responses)

| 功能 | 状态 | 实现位置 | 测试覆盖 |
|------|------|---------|---------|
| `choices[0].message` → `output` | ✅ | `internal/converter/converter.go:155-211` | ✅ |
| `tool_calls` → `function_call` output | ✅ | `internal/converter/converter.go:184-196` | ✅ |
| `usage` 转换 | ✅ | `internal/converter/converter.go:202-209` | ✅ |
| `response.id` 生成 | ✅ | `internal/converter/converter.go:157` | ✅ |

## 流式响应 (SSE Events)

| 事件 | 状态 | 实现位置 |
|------|------|---------|
| `response.created` | ✅ | `internal/converter/streaming.go:67-75` |
| `response.output_item.added` | ✅ | `internal/converter/streaming.go:156-169, 202-207` |
| `response.output_text.delta` | ✅ | `internal/converter/streaming.go:171-178` |
| `response.output_item.done` | ✅ | `internal/converter/streaming.go:104-126` |
| `response.completed` | ✅ | `internal/converter/streaming.go:128-137` |

## 存储功能

| 功能 | 状态 | 实现位置 | 测试覆盖 |
|------|------|---------|---------|
| BBolt 持久化 | ✅ | `internal/storage/storage.go` | ✅ |
| `Store(responseID, messages)` | ✅ | `internal/storage/storage.go:41-52` | ✅ |
| `Get(responseID)` | ✅ | `internal/storage/storage.go:54-73` | ✅ |
| `Delete(responseID)` | ✅ | `internal/storage/storage.go:75-81` | ✅ |
| 多模态内容存储 | ✅ | - | ✅ |
| Tool calls 存储 | ✅ | - | ✅ |
| 重启后数据保留 | ✅ | - | ✅ |

## 测试覆盖

| 测试文件 | 状态 | 测试数 |
|---------|------|--------|
| `internal/storage/storage_test.go` | ✅ | 6 |
| `internal/converter/converter_test.go` | ✅ | 10 |

## 未实现功能 (非必需)

| 功能 | 说明 |
|------|------|
| `text.format` (Structured Outputs) | 需要 Chat Completions 支持 |
| `web_search` tool | 上游提供商支持 |
| `file_search` tool | 上游提供商支持 |
| `code_interpreter` tool | 上游提供商支持 |
| `reasoning.encrypted_content` | ZDR 场景 |

## 运行测试

```bash
go test ./... -v
```

## 验证多轮对话

```bash
# 1. 启动服务
./bin/responses2chat -c ./configs/config.yaml

# 2. 第一轮对话
curl -X POST http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{"model": "gpt-4", "input": [{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "My name is Alice"}]}]}'
# 记录返回的 id，如 resp-xxx

# 3. 第二轮对话（带历史）
curl -X POST http://localhost:8080/v1/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -d '{"model": "gpt-4", "previous_response_id": "resp-xxx", "input": [{"type": "message", "role": "user", "content": [{"type": "input_text", "text": "What is my name?"}]}]}'
# 应返回: Your name is Alice

# 4. 查询历史
curl http://localhost:8080/v1/responses/resp-xxx
# 应返回完整对话记录
```
