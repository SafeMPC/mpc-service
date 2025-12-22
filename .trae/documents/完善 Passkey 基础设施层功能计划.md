# 完善 Passkey 基础设施层功能计划

## 1. API 定义更新 (Swagger)
### `api/definitions/infra.yml`
- **更新 `AuthToken`**: 增加 `passkey_signature`, `authenticator_data`, `client_data_json`, `credential_id` 字段。
- **新增 `PostAddUserPasskeyPayload`**: 定义注册 Passkey 的请求体 (UserID, CredentialID, PublicKey, DeviceName)。
- **新增 `AddUserPasskeyResponse`**: 定义注册响应。

### `api/paths/infra.yml`
- **新增 `/api/v1/infra/passkeys`**: 添加 `POST` 方法用于注册用户 Passkey。

## 2. 代码生成
- 运行 `make swagger`: 更新 Swagger 定义并生成 Go 类型代码 (`internal/types`)。
- 运行 `make go-generate-handlers`: 生成 HTTP Handler 绑定代码。

## 3. 内部服务层实现
### `internal/infra/signing`
- **更新 `types.go`**: 同步更新内部 `AuthToken` 结构体，包含 Passkey 字段。
- **更新 `service.go`**: 修改 `ThresholdSign` 方法，将 `AuthToken` 中的 Passkey 字段映射到 gRPC 的 `StartSignRequest`。

### `internal/infra/key`
- **更新 `service.go`**: 实现 `AddUserPasskey` 方法，调用 `metadataStore.SaveUserPasskey` 存储用户 Passkey 信息。

## 4. API Handler 实现
### 现有 Handler 更新
- **`internal/api/handlers/infra/signing/post_sign.go`**: 更新绑定逻辑，从请求中解析新增的 Passkey 字段并传递给 Service。

### 新增 Handler
- **`internal/api/handlers/infra/users/post_add_passkey.go`**: 实现 Passkey 注册接口，调用 `keyService.AddUserPasskey`。

## 5. 验证
- 运行 `go build ./...` 确保编译通过。
- 验证 API 接口参数绑定和 Service 调用链路是否打通。