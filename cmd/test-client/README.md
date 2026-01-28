# MPC Service Test Client

用于测试 MPC Service 的 CLI 工具，模拟 iOS 客户端的核心功能。

## 功能

- WebAuthn 注册和登录
- 创建钱包（DKG）
- 签名交易
- WebSocket 消息中继
- MPC 协议消息处理（模拟）

## 编译

```bash
cd mpc-service
go build -o bin/test-client ./cmd/test-client
```

或在容器内：

```bash
docker compose exec mpc-service bash
cd /app
go build -o bin/test-client ./cmd/test-client
```

## 使用方法

### 完整流程测试

测试从注册到签名的完整流程：

```bash
./bin/test-client --test full --user-id test-user
```

### 仅测试 WebAuthn

```bash
./bin/test-client --test webauthn --user-id test-user
```

### 仅测试 DKG（创建钱包）

```bash
./bin/test-client --test dkg --user-id test-user
```

### 仅测试签名

```bash
./bin/test-client --test sign --user-id test-user --wallet-id <wallet-id>
```

### 自定义服务地址

```bash
./bin/test-client --url http://localhost:8080 --ws-url ws://localhost:8080 --test full
```

### 启用详细日志

```bash
./bin/test-client --test full --verbose
```

## 命令行参数

| 参数 | 说明 | 默认值 |
|------|------|--------|
| `--url` | REST API 基础 URL | `http://localhost:8080` |
| `--ws-url` | WebSocket URL | `ws://localhost:8080` |
| `--test` | 测试类型：`webauthn`, `dkg`, `sign`, `full` | `full` |
| `--user-id` | 用户 ID | `test-user` |
| `--wallet-id` | 钱包 ID（用于签名测试） | 空 |
| `--verbose` | 启用详细日志 | `false` |

## 测试流程说明

### 1. WebAuthn 测试

1. 调用 `/v1/auth/webauthn/register/begin` 开始注册
2. 模拟前端创建凭证（使用模拟数据）
3. 调用 `/v1/auth/webauthn/register/finish` 完成注册
4. 调用 `/v1/auth/webauthn/login/begin` 开始登录
5. 模拟前端获取断言（使用模拟数据）
6. 调用 `/v1/auth/webauthn/login/finish` 完成登录并获取 JWT Token

**注意**: 由于 WebAuthn 需要浏览器支持，这里使用模拟数据。真实的 WebAuthn 测试需要使用浏览器。

### 2. DKG 测试

1. 先执行 WebAuthn 登录获取 JWT Token
2. 调用 `POST /v1/wallets` 创建钱包（触发 DKG）
3. 通过 WebSocket 连接参与 DKG 协议
4. 等待 DKG 完成

### 3. 签名测试

1. 先执行 WebAuthn 登录获取 JWT Token
2. 调用 `POST /v1/wallets/{id}/sign` 发起签名请求
3. 通过 WebSocket 连接参与签名协议
4. 等待签名完成

### 4. 完整流程测试

依次执行：
1. WebAuthn 注册
2. WebAuthn 登录
3. 创建钱包（DKG）
4. 签名交易

## 限制

1. **WebAuthn 模拟**: 由于 WebAuthn 需要硬件支持（Passkey），CLI 工具使用模拟数据。真实的 WebAuthn 测试需要使用浏览器。

2. **MPC 协议模拟**: CLI 工具使用模拟的协议消息，而不是真实的 tss-lib 实现。这主要用于测试消息中继功能。

3. **Client 签名**: CLI 工具使用模拟的 Client 签名，而不是真实的 Passkey 签名。

## 开发

### 添加新的测试功能

1. 在相应的 `*_client.go` 文件中添加新的方法
2. 在 `main.go` 中添加新的测试函数
3. 更新命令行参数（如需要）

### 集成真实的 tss-lib

如果需要集成真实的 tss-lib：

1. 在 `mpc_client.go` 中导入 tss-lib
2. 替换 `SimulateDKGMessage` 和 `SimulateSignMessage` 为真实的协议实现
3. 实现真实的 Client 签名（使用 Passkey 私钥）

## 故障排除

### 连接失败

- 检查服务是否运行：`docker compose ps`
- 检查端口是否正确：`curl http://localhost:8080/health`

### WebAuthn 测试失败

- 检查后端是否正确配置了 WebAuthn
- 检查数据库中的 passkeys 表是否存在
- 查看服务日志：`docker compose logs mpc-service`

### WebSocket 连接失败

- 检查 WebSocket URL 是否正确
- 检查 JWT Token 是否有效
- 检查 session_id 是否正确

## 示例输出

```
=== Testing Full Flow ===
Step 1: WebAuthn Registration
INFO: Starting WebAuthn registration...
INFO: WebAuthn registration completed

Step 2: WebAuthn Login
INFO: Starting WebAuthn login...
INFO: WebAuthn login completed token=eyJhbGciOiJIUzI1NiIs...

Step 3: Create Wallet (DKG)
INFO: Creating wallet (DKG)...
INFO: Wallet created successfully wallet_id=wallet-123...

Step 4: Sign Transaction
INFO: Signing transaction...
INFO: Transaction signed successfully signature=0x1234...
```
