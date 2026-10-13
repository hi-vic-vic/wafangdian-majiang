# Go 实时服务

这是瓦房店推倒胡的服务端起点。它使用内存保存房间，适合本地原型和内测，不适合多实例生产部署。

## 启动

```sh
cd /Users/lmc10242/working/tools/Codex/wafangdian-majiang
./scripts/backend.sh start
```

当前联机服务使用 `cmd/local`，监听 `http://127.0.0.1:8081`，可用 `GET /healthz` 检查状态。停止、重启及日志命令参见 [本地联机说明](../LOCAL-TEST.md)。

下方 API 说明对应此前的 `cmd/server` 骨架。当前四人联机请使用 `internal/live` 与上述启动脚本。

## 当前 API

- `POST /v1/rooms` 创建房间，传入 `hostId`、`hostName` 和规则配置。
- `POST /v1/rooms/{code}/join` 加入未开局房间。
- `GET /v1/rooms/{code}` 获取权威房间状态。
- `GET /v1/rooms/{code}/ws?playerId=...` 建立 WebSocket 连接。

WebSocket 客户端可发送：

```json
{ "type": "ready", "ready": true }
```

或：

```json
{ "type": "discard", "tile": "1m" }
```

服务端会广播 `room_state`，并拒绝未轮到当前玩家的出牌。

## 下一步

需要把完整牌墙、发牌、碰杠胡仲裁、超时托管、断线重连和结算账本接入同一服务端状态机。正式部署时将内存房间替换为 Redis 和 PostgreSQL，并接入微信登录校验。
