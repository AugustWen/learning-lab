# MCP 实验

通过 stdio 连接的 MCP Server 和 Client，提供 `greet` 工具。依赖由 `agent-study/go.mod` 管理。

在 `agent-study` 目录下运行：

```sh
go build -o ./bin/server ./mcp/server
go run ./mcp/client
```

Client 会启动 Server 并调用 `greet`，传入 `{"name":"you"}`，输出问候结果。修改 Server 后需要重新构建。
