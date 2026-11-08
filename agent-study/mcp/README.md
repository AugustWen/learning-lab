# MCP 实验

通过 stdio 连接的 MCP Server 和 Client，提供 `add` 和 `multiply` 工具。依赖由 `agent-study/go.mod` 管理。

在 `agent-study` 目录下运行：

```sh
go build -o ./bin/server ./mcp/server
go run ./mcp/client
```

Client 会启动 Server 并调用 `add`，传入 `{"a":2,"b":3}`，输出结果 `{"ok":true,"data":5}`。修改 Server 后需要重新构建。
