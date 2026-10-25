# Agent 学习实验

本目录使用独立的 Go 模块，依赖与 `go-study` 分开管理。

- `agent-loop/`：复制自 `go-study/llm-basics/02-tool-calling`，包含现有实现和测试。
- `mcp/`：预留 MCP 实验目录。
- `skills/`：预留 Skills 实验目录。
- `internal/config/`：本模块使用的配置加载代码。

在 `agent-study` 目录下运行：

```sh
cp config.example.json config.json
# 在 config.json 中填入自己的 OpenRouter API Key
go run ./agent-loop
```

运行测试（不需要 API Key）：

```sh
go test ./...
```

`config.json` 已被 Git 忽略。原有 `go-study/llm-basics` 保留不变。
