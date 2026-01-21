# mpc-service API 状态

## ✅ 新的 API 定义（V2）

### Paths（API 路径）
- ✅ `api/paths/auth.yml` - 用户认证（注册、登录）
- ✅ `api/paths/webauthn.yml` - WebAuthn 认证（4个接口）
- ✅ `api/paths/wallets.yml` - 钱包管理（7个接口）⭐ 新增
- ✅ `api/paths/sessions.yml` - 会话查询（1个接口）⭐ 新增
- ✅ `api/paths/common.yml` - 系统接口（健康检查等）
- ✅ `api/paths/push.yml` - 推送通知
- ✅ `api/paths/well_known.yml` - Well-known 路径
- ❌ `api/paths/infra.yml` - 已废弃（被 wallets.yml 替代）

### Definitions（类型定义）
- ✅ `api/definitions/auth.yml` - 认证类型
- ✅ `api/definitions/webauthn.yml` - WebAuthn 类型 ⭐ 新增
- ✅ `api/definitions/wallets.yml` - 钱包类型 ⭐ 新增
- ✅ `api/definitions/sessions.yml` - 会话类型 ⭐ 新增
- ✅ `api/definitions/common.yml` - 通用类型
- ✅ `api/definitions/errors.yml` - 错误类型
- ⚠️ `api/definitions/infra.yml` - 保留部分定义（清理中）

---

## 📋 API 对比

### 旧接口（infra.yml）
```
POST /v1/infra/keys          → 废弃
GET  /v1/infra/keys          → 废弃
POST /v1/infra/sign          → 废弃
... (40+ 个接口)
```

### 新接口（wallets.yml + sessions.yml）
```
POST /v1/wallets             → 创建钱包
GET  /v1/wallets             → 列出钱包
GET  /v1/wallets/{id}        → 查询钱包
POST /v1/wallets/{id}/sign   → 签名交易
... (11 个核心接口)
```

---

## 🎯 下一步

1. 删除 `api/paths/infra.yml.backup`
2. 清理 `api/definitions/infra.yml`（保留必要的类型）
3. 生成 Swagger 并测试

---

**新接口更清晰、更符合用户视角！** ✨
