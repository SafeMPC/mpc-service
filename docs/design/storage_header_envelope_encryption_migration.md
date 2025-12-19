# 存储头部格式 + 逐文件随机 salt + 信封加密 改造方案

## 目标
- 提升本地加密文件（`MPC share` 与 `LocalPartySaveData`）的抗攻击能力与可运维性
- 引入统一的“文件头部格式”，支持逐文件随机 `salt`、逐文件随机数据密钥（DEK）与信封加密（DEK/KEK）
- 保持向后兼容，提供平滑迁移与回滚策略

## 核心设计
- 逐文件随机 `salt`：每个文件独立 `salt`，抵御跨文件预计算和字典复用
- 逐文件随机 DEK：每个文件生成独立 256-bit DEK 用于内容加密（AES-256-GCM）
- 信封加密（Envelope Encryption）：DEK 不以明文存储，使用 KEK 包裹（KMS/HSM 或密码派生）
- 统一文件头部：携带版本、KDF 参数、随机 `salt`、包裹后的 DEK、`nonce`、元信息与可扩展字段
- AEAD 绑定：头部部分字段作为 AAD，提高整体完整性与抗篡改性

## 文件头部格式
- Magic：`"MPCENC\0"` 8 字节
- Version：`uint16`（初始 `1`）
- Flags：`uint16`（保留扩展）
- ContentType：`uint8`（1=`MPC_SHARE`，2=`LOCAL_PARTY_SAVE_DATA`）
- KDFSection：
  - KDFId：`uint8`（1=`scrypt`）
  - N：`uint32`，r：`uint32`，p：`uint32`
  - SaltLen：`uint16`；Salt：`bytes[SaltLen]`（随机生成）
- EnvelopeSection：
  - Mode：`uint8`（1=`KMS_KEK`，2=`Password_KEK`）
  - KEKIdLen：`uint16`；KEKId：`bytes[KEKIdLen]`（指纹/ARN/租户ID）
  - WrappedDEKLen：`uint16`；WrappedDEK：`bytes[WrappedDEKLen]`（DEK 包裹结果）
- AEADSection：
  - NonceLen：`uint8`；Nonce：`bytes[NonceLen]`（GCM 随机）
  - AADLen：`uint16`；AAD：`bytes[AADLen]`（可为空；建议包含 `Magic+Version+Flags+ContentType+KEKId`）
- MetaSection（可选）：
  - KeyIdLen：`uint16`；KeyId：`bytes[...]`
  - NodeIdLen：`uint16`；NodeId：`bytes[...]`
  - ProtocolLen：`uint8`；Protocol：`bytes[...]`（如 `gg20`/`frost`）
  - CurveLen：`uint8`；Curve：`bytes[...]`（如 `secp256k1`）
  - CreatedAt：`int64`（epoch）
- Ciphertext：`bytes[...]`（AES-GCM 输出，包含 Tag；`Nonce`在头部管理）

说明：
- 头部采用 TLV/长度前缀结构，便于解析与扩展
- AAD 绑定头部关键字段，防止拆头换头攻击

## 加密流程
- 生成随机 DEK（256-bit）
- 生成随机 `salt` 与 `nonce`
- 根据 Envelope 模式包裹 DEK：
  - KMS 模式：`WrappedDEK = KMS.Encrypt(DEK)`，`KEKId` 为 KMS Key 标识
  - 密码模式：`KEK = KDF(encryptionKey, salt)`，`WrappedDEK = AES-KWP(KEK, DEK)` 或 `AES-GCM` 包裹
- 使用 DEK + `nonce` 对明文执行 AES-GCM 加密；设置 AAD
- 写入头部与密文，原子重命名

## 解密流程
- 解析头部，读取 Envelope 模式与 `WrappedDEK`
- 解包 DEK：
  - KMS 模式：`DEK = KMS.Decrypt(WrappedDEK)`
  - 密码模式：`DEK = AES-KWP-unwrap(KDF(encryptionKey, salt), WrappedDEK)`
- 使用 DEK + `nonce` 解密密文；设置相同 AAD 验证完整性

## 密钥管理
- KEK（主密钥）：
  - KMS/HSM：推荐；不出宿主机内存与服务域，支持轮转与审计
  - 密码派生：保留兼容模式；强制高熵口令与独立租户盐策略
- DEK（数据密钥）：
  - 每文件随机生成；不共享；轮转时仅重包裹，无需解密明文重写

## 轮转与恢复
- KEK 轮转：
  - KMS 模式：读取头部，`KMS.ReEncrypt(WrappedDEK)`到新 KEK；无需触碰密文
  - 密码模式：读取头部，用旧口令解包 DEK，用新口令与新 `salt` 重新包裹
- 恢复：
  - SSS 备份恢复不依赖本地密钥配置；恢复出 `MPC share` 后按新方案写盘（生成新 DEK/盐/头部）

## 兼容与迁移
- 读：
  - 优先检测 `Magic`；如存在按新格式解析
  - 如无 `Magic`，走旧版路径：`scrypt(encryptionKey, fixed salt)` → AES-GCM 解密
- 写：
  - 新文件全部使用新格式
  - 迁移工具支持批量重加密旧文件为新格式
- 回滚：
  - 保留旧版读支持；必要时迁移工具可反向输出旧格式（仅临时用途）

## 安全与运维
- 秘钥与盐：
  - `salt` 每文件随机；`nonce` 每加密操作随机；`encryptionKey` 强熵与最小权限
- 审计：
  - 记录 `KeyId/NodeId/KEKId/Version/CreatedAt/Operator` 与操作类型（写入/轮转/恢复）
- 原子写与并发：
  - 统一临时文件写入 + `rename`；文件锁保护并发写

## 改造范围与接口变更
- 存储层（读写）
  - 新增头部编解码模块；引入 Envelope（KMS/密码）抽象
- 配置
  - `MPC_KEY_SHARE_ENCRYPTION_KEY` 保留（密码模式）
  - 新增 `MPC_KMS_KEY_ID`、`MPC_KMS_ENDPOINT`、`MPC_ENCRYPTION_MODE`（`kms`/`password`）
- 迁移工具
  - 扫描旧文件 → 解密 → 按新格式重加密；并生成迁移报告与审计条目

## 演进计划
- Phase 0：原型与文件头部编码实现；新增读写路径与单测
- Phase 1：引入 KMS/密码两种 Envelope 实现；配置切换
- Phase 2：迁移工具与回滚工具；灰度迁移与演练
- Phase 3：默认使用新格式；旧读逻辑保留一段时间后清理

## 风险与缓解
- 旧文件不可解密：在迁移前强制“仅外部分片”的恢复演练，并保留回滚方案
- KMS 不可用：降级到密码模式或启用本地缓存与重试策略；审计记录
- 回滚需求：保留旧读逻辑与双写选项（短期），明确清理窗口

---
本方案为设计文档，暂不进行代码改动或接口新增。 
