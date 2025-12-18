# 详细设计文档：2-of-3 Delegated Guardian 与 团队多签实现

## 1. 概述
本文档基于 `docs/design/2_of_3_delegated_guardian.md` 的架构设计，结合 `go-mpc-wallet` 现有代码库，提供具体的落地实施细节。

核心目标：
1.  实现 **Delegated Guardian** 模式：用户通过 APP 签名授权，Guardian 节点验证通过后才参与 MPC 签名。
2.  支持 **团队多签**：Guardian 支持配置“N-of-M”策略，收集齐 N 个团队成员签名后才放行。

## 2. 系统架构变更

### 2.1 现有架构 (AS-IS)
*   **API Layer**: 接收 `POST /sign` 请求。
*   **MPC Coordinator**: 协调各节点启动签名会话。
*   **MPC Node**: 收到 `StartSign` gRPC 请求后，直接加载密钥分片进行计算。
*   **Database**: 存储密钥元数据、用户信息。

### 2.2 目标架构 (TO-BE)
*   **MPC Node (Guardian)**:
    *   新增 **策略引擎 (Policy Engine)**：拦截 `StartSign` 请求。
    *   新增 **鉴权模块 (Auth Module)**：验证 `AuthToken`（用户签名）。
*   **Database**:
    *   新增 `user_auth_keys` 表：存储用户/团队成员的公钥。
    *   新增 `signing_policies` 表：存储钱包的鉴权策略（单人 vs 团队，阈值等）。
*   **API Protocol**:
    *   `StartSignRequest` (gRPC) 新增 `auth_tokens` 字段。

## 3. 数据库设计 (PostgreSQL)

### 3.1 用户鉴权公钥表 (`user_auth_keys`)
用于存储用户 APP 端生成的控制公钥（非 MPC 分片）。

```sql
CREATE TABLE user_auth_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    wallet_id VARCHAR(255) NOT NULL, -- 关联的 MPC 钱包 ID (KeyID)
    public_key_hex VARCHAR(512) NOT NULL, -- 用户公钥 (Hex 编码)
    key_type VARCHAR(50) NOT NULL DEFAULT 'ed25519', -- 密钥类型: ed25519, secp256k1
    member_name VARCHAR(100), -- 团队成员名称 (可选，用于审计)
    role VARCHAR(50) DEFAULT 'member', -- 角色: owner, admin, member
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    
    CONSTRAINT uk_wallet_pubkey UNIQUE (wallet_id, public_key_hex)
);
CREATE INDEX idx_user_auth_keys_wallet ON user_auth_keys(wallet_id);
```

### 3.2 签名策略表 (`signing_policies`)
定义每个钱包的鉴权规则。

```sql
CREATE TABLE signing_policies (
    wallet_id VARCHAR(255) PRIMARY KEY, -- 关联 KeyID
    policy_type VARCHAR(50) NOT NULL DEFAULT 'single', -- 'single' (单人), 'team' (团队多签)
    min_signatures INT NOT NULL DEFAULT 1, -- 最小所需签名数
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

## 4. 接口协议变更 (Protobuf)

### 4.1 修改 `mpc.proto`

文件路径: `internal/pb/mpc/v1/mpc.proto`

需要在 `StartSignRequest` 中增加鉴权令牌字段。为了支持团队多签，使用 `repeated` 列表。

```protobuf
message AuthToken {
    string public_key = 1; // 签名者的公钥 (用于快速查找)
    bytes signature = 2;   // 对本次交易 (message/message_hex) 的签名
    string member_id = 3;  // 可选：成员标识
}

message StartSignRequest {
    // ... 现有字段 ...
    string session_id = 1;
    string key_id = 2;
    bytes message = 3;
    // ...
    
    // [新增] 鉴权令牌列表
    repeated AuthToken auth_tokens = 11;
}
```

## 5. 核心逻辑实现

### 5.1 策略引擎 (Policy Engine)
位于 `internal/mpc/grpc/server.go` 的 `StartSign` 方法中。

**伪代码逻辑：**

```go
func (s *GRPCServer) StartSign(ctx context.Context, req *pb.StartSignRequest) (*pb.StartSignResponse, error) {
    // 1. 判断是否开启 Guardian 模式
    if s.config.IsGuardianNode {
        // 2. 加载策略
        policy, err := s.store.GetPolicy(req.KeyId)
        if err != nil {
            // 默认回退到单人模式或报错，视安全配置而定
            return nil, status.Errorf(codes.Internal, "failed to load policy")
        }

        // 3. 加载允许的公钥列表
        allowedKeys, err := s.store.ListUserAuthKeys(req.KeyId)
        
        // 4. 验证签名
        validSigCount := 0
        visitedKeys := make(map[string]bool)

        for _, token := range req.AuthTokens {
            // A. 公钥白名单检查
            if !isAllowed(token.PublicKey, allowedKeys) {
                continue
            }
            
            // B. 防重放/去重
            if visitedKeys[token.PublicKey] {
                continue
            }

            // C. 验签 (Verify Signature)
            // 验证 token.Signature 是否是对 req.Message 的有效签名
            if verify(token.PublicKey, req.Message, token.Signature) {
                validSigCount++
                visitedKeys[token.PublicKey] = true
            }
        }

        // 5. 阈值检查
        if validSigCount < policy.MinSignatures {
             return &pb.StartSignResponse{
                Started: false,
                Message: fmt.Sprintf("Access Denied: need %d signatures, got %d", policy.MinSignatures, validSigCount),
            }, nil
        }
        
        log.Info("Guardian check passed")
    }

    // ... 继续执行原有的 MPC StartSign 逻辑 ...
}
```

### 5.2 API 层变更
用户提交交易时，需要先在 APP 端完成签名，然后通过 API 传给后端。

**请求结构体 (`internal/types`) 更新：**

```go
type SignTransactionRequest struct {
    // ... 现有字段 ...
    
    // 新增
    AuthTokens []struct {
        PublicKey string `json:"public_key"`
        Signature string `json:"signature_hex"`
    } `json:"auth_tokens"`
}
```

**协调者 (`Coordinator`) 逻辑：**
Coordinator 收到 API 请求后，将 `AuthTokens` 封装进 `StartSignRequest`，然后广播给所有 MPC 节点（包括 Guardian 节点）。

## 6. 开发计划与步骤

1.  **Phase 1: 数据层 (Day 1)**
    *   创建 SQL 迁移脚本，建立 `user_auth_keys` 和 `signing_policies` 表。
    *   使用 SQLBoiler 生成 Go Model 代码。

2.  **Phase 2: 协议层 (Day 1)**
    *   修改 `mpc.proto`，添加 `AuthToken` 定义。
    *   重新编译 Protobuf 生成 Go 代码。

3.  **Phase 3: 业务逻辑 (Day 2-3)**
    *   在 `StartSign` 中实现拦截逻辑。
    *   实现 `verify` 函数（支持 Ed25519 和 Secp256k1 普通验签）。
    *   修改 API Handler 和 Coordinator，透传 `AuthTokens`。

4.  **Phase 4: 测试 (Day 4)**
    *   单元测试：测试策略引擎在不同阈值下的行为。
    *   集成测试：模拟 APP 端签名，验证端到端流程。

## 7. 安全注意事项
1.  **重放攻击**：APP 签名的消息体 (`message`) 必须包含 Nonce 或 Timestamp，或者是唯一的交易 Hash。
2.  **公钥绑定**：用户注册公钥时，必须进行严格的身份验证（KYC），防止攻击者注册自己的公钥。
3.  **日志脱敏**：不要在日志中打印 `AuthToken` 的具体内容。
