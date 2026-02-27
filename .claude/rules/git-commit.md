# Git Commit 提交前校验规则

## 强制要求

在执行 `git commit` 之前，**必须**完成以下检查：

### 1. 检查暂存区内容

```bash
git diff --staged
```

必须逐行审查所有变更，确认没有以下内容：

- API Key（如 `sk-xxx`、`Bearer xxx`）
- 密码、Token、Secret
- 数据库连接字符串
- 私钥文件内容
- `.env` 文件或类似的配置文件
- 包含硬编码凭证的测试脚本

### 2. 检查暂存的文件列表

```bash
git diff --staged --name-only
```

确认没有意外添加的文件，特别是：
- 测试脚本（`test_*.sh`）
- 配置文件（`config.yaml`、`config.local.yaml`）
- 环境文件（`.env*`）

### 3. 敏感信息模式匹配

如果发现以下模式，**禁止提交**：

| 模式 | 示例 |
|------|------|
| OpenAI/DeepSeek API Key | `sk-xxxxxxxx` |
| Bearer Token | `Bearer xxx` |
| Authorization Header | `Authorization: xxx` |
| 连接字符串 | `password=xxx`, `passwd=xxx` |
| 私钥标记 | `-----BEGIN PRIVATE KEY-----` |

## 如果发现敏感信息

1. **立即停止**，不要继续提交
2. 从暂存区移除该文件：`git restore --staged <file>`
3. 修改文件，移除敏感信息或使用环境变量替代
4. 重新执行上述检查流程

## 例外情况

只有在用户**明确要求**且**知晓风险**的情况下，才能跳过此检查。
