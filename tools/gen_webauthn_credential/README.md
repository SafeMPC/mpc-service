# WebAuthn 凭证生成工具

用于生成符合 WebAuthn 标准的测试凭证，支持注册和登录响应。

## 功能

- 生成 WebAuthn 注册响应（包含完整的 attestation object）
- 生成 WebAuthn 登录断言响应
- 生成 ECDSA P-256 密钥对
- 输出符合 WebAuthn 标准的 JSON 格式

## 使用方法

### 生成注册响应

```bash
# 1. 从服务器获取 challenge
CHALLENGE=$(curl -s -X POST http://localhost:8080/api/v1/auth/webauthn/register/begin \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"test-user","user_name":"test@example.com"}' | jq -r '.session_data')

# 2. 生成凭证响应
./bin/gen-webauthn-credential --action register --challenge "$CHALLENGE"
```

输出包含：
- 完整的 WebAuthn 注册响应（JSON 格式）
- Credential ID（保存用于登录）
- Private Key（保存用于登录签名）

### 生成登录响应

```bash
# 1. 从服务器获取 challenge
CHALLENGE=$(curl -s -X POST http://localhost:8080/api/v1/auth/webauthn/login/begin \
  -H 'Content-Type: application/json' \
  -d '{"user_id":"test-user"}' | jq -r '.session_data')

# 2. 使用之前注册的 credential ID 和私钥生成登录响应
./bin/gen-webauthn-credential --action login \
  --challenge "$CHALLENGE" \
  --credential-id "YOUR_CREDENTIAL_ID" \
  --privkey "YOUR_PRIVATE_KEY_HEX"
```

## 参数说明

| 参数 | 说明 | 必需 |
|------|------|------|
| `--action` | 操作类型：`register` 或 `login` | 是 |
| `--challenge` | 服务器返回的 challenge (base64url) | 是 |
| `--credential-id` | Credential ID (仅用于 login，base64url) | login 时必需 |
| `--privkey` | 私钥 (hex 格式，仅用于 login) | login 时必需 |
| `--origin` | Origin URL | 否（默认：http://localhost:8080） |
| `--rp-id` | Relying Party ID | 否（默认：localhost） |

## 输出格式

### 注册响应

```json
{
  "id": "credential-id-base64url",
  "rawId": "credential-id-base64url",
  "type": "public-key",
  "response": {
    "clientDataJSON": "base64url...",
    "attestationObject": "base64url..."
  }
}
```

### 登录响应

```json
{
  "id": "credential-id-base64url",
  "rawId": "credential-id-base64url",
  "type": "public-key",
  "response": {
    "clientDataJSON": "base64url...",
    "authenticatorData": "base64url...",
    "signature": "base64url...",
    "userHandle": null
  }
}
```

## 注意事项

1. **测试用途**：此工具生成的凭证用于测试目的，不能替代真实的浏览器/设备生成的 Passkey

2. **格式验证**：生成的凭证格式符合 WebAuthn 标准，但可能无法通过所有验证（特别是 attestation 验证）

3. **真实测试**：生产环境测试应使用真实的浏览器 WebAuthn API 或移动设备的 Passkey API

4. **私钥安全**：生成的私钥仅用于测试，不应在生产环境中使用

## 技术细节

- 使用 ECDSA P-256 曲线
- 使用 "none" attestation 格式（无证明）
- COSE 格式的公钥编码
- CBOR 编码的 attestation object
- ASN.1 DER 格式的签名

## 集成到测试客户端

测试客户端 (`cmd/test-client`) 已集成此工具，可以自动生成凭证用于测试。
