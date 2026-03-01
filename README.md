# Clip Registry

Discover clips across Pinix Servers.

Registry 是一种 Clip，帮助 Clip Client 发现任意 Pinix Server 上有哪些 Clip Instance 可用。

## Commands

| Command | 说明 |
|---------|------|
| `list` | 列出目标 Server 的所有 Clip（支持指定 server 或查全部） |
| `add-server` | 添加 Server 连接配置 |
| `remove-server` | 移除 Server 连接配置 |
| `list-servers` | 列出已配置的 Server（token 仅显示 hint） |

## 使用方式

通过 `pinix-clip invoke` 调用：

```bash
# 添加 Server
pinix-clip invoke registry add-server --stdin '{"name":"home","host":"100.66.47.40","port":9875,"token":"your-super-token"}'

# 列出所有 Server 的 Clip
pinix-clip invoke registry list

# 列出指定 Server 的 Clip
pinix-clip invoke registry list --stdin '{"server":"home"}'

# 查看已配置的 Server
pinix-clip invoke registry list-servers

# 移除 Server
pinix-clip invoke registry remove-server --stdin '{"name":"home"}'
```

## 依赖

- `bash`
- `jq`
- `curl`
