# WebAuthn Service

WebAuthn/Passkey 认证服务，用于 MPC 钱包的无密码认证。

## 功能

- Passkey 注册（创建凭证）
- Passkey 登录（验证凭证）
- 断言验证（关键操作的二次验证）

## 使用

### 注册流程

```go
// 1. 开始注册
options, sessionData, err := webauthnService.BeginRegistration(ctx, userID, userName)

// 2. 前端使用 options 调用 navigator.credentials.create()

// 3. 完成注册
err = webauthnService.FinishRegistration(ctx, userID, userName, sessionData, credentialResponse)
```

### 登录流程

```go
// 1. 开始登录
options, sessionData, err := webauthnService.BeginLogin(ctx, userID)

// 2. 前端使用 options 调用 navigator.credentials.get()

// 3. 完成登录
err = webauthnService.FinishLogin(ctx, userID, sessionData, assertionResponse)
```

## 数据库

Passkey 数据存储在 `passkeys` 表：

```sql
CREATE TABLE passkeys (
    credential_id VARCHAR(512) PRIMARY KEY,
    public_key TEXT NOT NULL,
    device_name VARCHAR(255),
    created_at TIMESTAMPTZ DEFAULT NOW()
);
```

## 注意事项

1. **用户关联**：当前 passkeys 表没有 user_id 字段，需要通过其他方式关联用户
2. **Session 管理**：当前 sessionData 简化存储为 challenge，生产环境应使用 Redis
3. **凭证查询**：需要实现用户凭证的查询逻辑
