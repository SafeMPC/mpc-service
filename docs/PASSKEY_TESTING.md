# Passkey 功能系统测试指南

本文档指导如何对系统的 Passkey 注册和签名功能进行端到端测试。

## 1. 准备工作

### 1.1 启动服务
确保系统所有组件已在 Docker 中启动并运行：
```bash
docker compose up -d
```

### 1.2 编译测试辅助工具
我们需要一个辅助工具来模拟 WebAuthn 客户端生成密钥对和签名数据。
```bash
# 编译工具
go build -o tools/gen_passkey_test_data/gen_passkey tools/gen_passkey_test_data/main.go
```

## 2. 测试步骤

### 步骤 1: 生成 Passkey 密钥对
运行工具生成一个新的 EC 密钥对和 Credential ID。

```bash
./tools/gen_passkey_test_data/gen_passkey -action keygen
```

**输出示例** (请保存好这些值，后续步骤需要用到):
```json
{
  "credential_id": "CRED_ID_XYZ...",
  "private_key": "PRIV_KEY_HEX...",
  "public_key": "04......"
}
```

我们将这些值设为环境变量以便后续使用：
```bash
export TEST_USER_ID="user-test-001"
export TEST_CRED_ID="<填入上面的 credential_id>"
export TEST_PUB_KEY="<填入上面的 public_key>"
export TEST_PRIV_KEY="<填入上面的 private_key>"
export TEST_DEVICE="test-device-mac"
```

### 步骤 2: 注册 Passkey
调用 REST API 将生成的公钥注册到系统。

```bash
curl -X POST http://localhost:8080/api/v1/infra/passkeys \
  -H "Content-Type: application/json" \
  -d "{
    \"credential_id\": \"$TEST_CRED_ID\",
    \"public_key\": \"$TEST_PUB_KEY\",
    \"device_name\": \"$TEST_DEVICE\"
  }"
```

**预期结果**:
```json
{"success":true,"message":"User passkey added successfully"}
```

### 步骤 3: 准备签名数据
我们需要模拟一次签名操作。假设我们要签名的消息是 "Hello World"。

1. 准备消息 Hex:
```bash
# "Hello World" 的 Hex 是 48656c6c6f20576f726c64
export TEST_MSG_HEX="48656c6c6f20576f726c64"
```

2. 使用工具生成 WebAuthn 签名数据:
```bash
./tools/gen_passkey_test_data/gen_passkey -action sign \
  -privkey $TEST_PRIV_KEY \
  -msg $TEST_MSG_HEX
```

**输出示例**:
```json
{
  "authenticator_data": "SZ...",
  "client_data_json": "eyJ...",
  "passkey_signature": "MEQ..."
}
```

我们将这些输出值设为环境变量：
```bash
export TEST_AUTH_DATA="<填入上面的 authenticator_data>"
export TEST_CLIENT_DATA="<填入上面的 client_data_json>"
export TEST_SIGNATURE="<填入上面的 passkey_signature>"
```

### 步骤 4: 发起签名请求 (验证 Passkey)
现在调用 MPC 签名接口，并在请求中附带 Passkey 验证数据。

*注意*: 您需要一个有效的 `key_id`。如果您还没有创建 MPC 密钥，请先创建一个（参考 `examples/management_usage` 或其他文档）。这里假设已有一个 `key_id`。

```bash
export TEST_KEY_ID="<填入现有的 KEY_ID>"
```

```bash
curl -X POST http://localhost:8080/api/v1/infra/sign \
  -H "Content-Type: application/json" \
  -d "{
    \"key_id\": \"$TEST_KEY_ID\",
    \"message_hex\": \"$TEST_MSG_HEX\",
    \"chain_type\": \"ethereum\",
    \"auth_tokens\": [
      {
        \"credential_id\": \"$TEST_CRED_ID\",
        \"passkey_signature\": \"$TEST_SIGNATURE\",
        \"authenticator_data\": \"$TEST_AUTH_DATA\",
        \"client_data_json\": \"$TEST_CLIENT_DATA\"
      }
    ]
  }"
```
*注*: `public_key` 和 `signature` 字段是旧版鉴权字段，在使用 Passkey 时可以为空或保留，系统优先验证 Passkey 字段。但为了兼容旧逻辑，建议 `public_key` 填入 Passkey 公钥。

**预期结果**:
如果 Passkey 验证通过，MPC 节点将开始签名流程，返回 `started: true`。
```json
{"started":true,"message":"Signing started in background"}
```

如果 Passkey 验证失败，日志中会出现 `Passkey signature verification failed`，API 可能返回错误或拒绝签名。

## 3. 故障排查

- **验证失败**: 检查 `coordinator` 和 `mpc-node` 的日志。
  ```bash
  docker compose logs -f coordinator mpc-node-1
  ```
- **Challenge 不匹配**: 确保签名时的 Challenge 是消息的 `Base64URL` 编码（工具已自动处理）。
- **User Not Present**: 工具默认设置了 UP 标志位 (0x01)，如果验证逻辑要求 UV (User Verified)，则需要在工具中修改 Flags。
