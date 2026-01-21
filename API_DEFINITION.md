# mpc-service API 完整定义

**版本**: v1.0  
**基于**: ARCHITECTURE_V2.md  
**目标**: MVP 核心功能

---

## 🎯 Service 的角色

### 对外（Client）
- REST API（用户操作）
- WebSocket（实时消息）

### 对内（Signer）
- gRPC Client（调用 Signer）

---

## 📝 REST API 定义

### Base URL
- 开发: `http://localhost:8080`
- 生产: `https://api.safempc.com`

### 认证方式
- `Authorization: Bearer <jwt>` - API 访问
- `webauthn_assertion: {...}` - 关键操作二次验证

---

## 1. 认证接口

### 1.1 WebAuthn 注册

#### 开始注册
```http
POST /v1/auth/webauthn/register/begin
Content-Type: application/json

Request:
{
  "email": "user@example.com",
  "display_name": "John Doe"
}

Response: 200 OK
{
  "user_id": "uuid",
  "options": {
    "challenge": "base64url...",
    "rp": {
      "name": "SafeMPC",
      "id": "safempc.com"
    },
    "user": {
      "id": "base64url...",
      "name": "user@example.com",
      "displayName": "John Doe"
    },
    "pubKeyCredParams": [...],
    "timeout": 60000,
    "attestation": "none",
    "authenticatorSelection": {
      "authenticatorAttachment": "platform",
      "userVerification": "required"
    }
  },
  "session_data": "base64url..."
}
```

#### 完成注册
```http
POST /v1/auth/webauthn/register/finish
Content-Type: application/json

Request:
{
  "user_id": "uuid",
  "session_data": "base64url...",
  "credential": {
    "id": "base64url...",
    "rawId": "base64url...",
    "type": "public-key",
    "response": {
      "attestationObject": "base64url...",
      "clientDataJSON": "base64url..."
    }
  }
}

Response: 200 OK
{
  "success": true,
  "access_token": "jwt...",
  "refresh_token": "jwt...",
  "expires_in": 3600
}
```

### 1.2 WebAuthn 登录

#### 开始登录
```http
POST /v1/auth/webauthn/login/begin
Content-Type: application/json

Request:
{
  "email": "user@example.com"
}

Response: 200 OK
{
  "user_id": "uuid",
  "options": {
    "challenge": "base64url...",
    "timeout": 60000,
    "rpId": "safempc.com",
    "allowCredentials": [
      {
        "type": "public-key",
        "id": "base64url..."
      }
    ],
    "userVerification": "required"
  },
  "session_data": "base64url..."
}
```

#### 完成登录
```http
POST /v1/auth/webauthn/login/finish
Content-Type: application/json

Request:
{
  "user_id": "uuid",
  "session_data": "base64url...",
  "assertion": {
    "id": "base64url...",
    "rawId": "base64url...",
    "type": "public-key",
    "response": {
      "authenticatorData": "base64url...",
      "clientDataJSON": "base64url...",
      "signature": "base64url...",
      "userHandle": "base64url..."
    }
  }
}

Response: 200 OK
{
  "success": true,
  "access_token": "jwt...",
  "refresh_token": "jwt...",
  "expires_in": 3600
}
```

### 1.3 令牌管理

```http
POST /v1/auth/refresh
Content-Type: application/json

Request:
{
  "refresh_token": "jwt..."
}

Response: 200 OK
{
  "access_token": "jwt...",
  "expires_in": 3600
}
```

```http
POST /v1/auth/logout
Authorization: Bearer <jwt>

Response: 200 OK
{
  "success": true
}
```

---

## 2. 钱包管理接口

### 2.1 创建钱包（DKG）

```http
POST /v1/wallets
Authorization: Bearer <jwt>
Content-Type: application/json

Request:
{
  "algorithm": "ECDSA",
  "curve": "secp256k1",
  "chain_type": "ethereum",
  "webauthn_assertion": {
    "credential_id": "base64url...",
    "authenticator_data": "base64url...",
    "client_data_json": "base64url...",
    "signature": "base64url..."
  }
}

Response: 201 Created
{
  "wallet_id": "uuid",
  "dkg_session_id": "uuid",
  "status": "pending",
  "websocket_url": "ws://localhost:8080/v1/ws?token=<session_token>"
}

说明:
- 必须提供 webauthn_assertion（二次验证）
- 返回 WebSocket URL 用于接收 DKG 协议消息
- Client 需要连接 WebSocket 并处理 MPC 消息
```

### 2.2 查询钱包

```http
GET /v1/wallets
Authorization: Bearer <jwt>
Query: ?chain_type=ethereum&limit=20&offset=0

Response: 200 OK
{
  "wallets": [
    {
      "wallet_id": "uuid",
      "address": "0x...",
      "public_key": "0x...",
      "chain_type": "ethereum",
      "algorithm": "ECDSA",
      "curve": "secp256k1",
      "created_at": "2025-01-21T10:00:00Z"
    }
  ],
  "total": 1,
  "limit": 20,
  "offset": 0
}
```

```http
GET /v1/wallets/{wallet_id}
Authorization: Bearer <jwt>

Response: 200 OK
{
  "wallet_id": "uuid",
  "address": "0x...",
  "public_key": "0x...",
  "chain_type": "ethereum",
  "algorithm": "ECDSA",
  "curve": "secp256k1",
  "created_at": "2025-01-21T10:00:00Z"
}
```

### 2.3 生成地址

```http
POST /v1/wallets/{wallet_id}/addresses
Authorization: Bearer <jwt>
Content-Type: application/json

Request:
{
  "derivation_path": "m/44'/60'/0'/0/0"
}

Response: 200 OK
{
  "address": "0x...",
  "derivation_path": "m/44'/60'/0'/0/0",
  "public_key": "0x..."
}
```

### 2.4 查询余额

```http
GET /v1/wallets/{wallet_id}/balance
Authorization: Bearer <jwt>

Response: 200 OK
{
  "balance": "1.5",
  "symbol": "ETH",
  "decimals": 18,
  "chain_type": "ethereum"
}
```

### 2.5 查询交易历史

```http
GET /v1/wallets/{wallet_id}/transactions
Authorization: Bearer <jwt>
Query: ?limit=20&offset=0

Response: 200 OK
{
  "transactions": [
    {
      "tx_hash": "0x...",
      "from": "0x...",
      "to": "0x...",
      "value": "1.5",
      "status": "confirmed",
      "timestamp": "2025-01-21T10:00:00Z"
    }
  ],
  "total": 10,
  "limit": 20,
  "offset": 0
}
```

---

## 3. 签名接口

### 3.1 签名交易

```http
POST /v1/wallets/{wallet_id}/sign
Authorization: Bearer <jwt>
Content-Type: application/json

Request:
{
  "message_hex": "0xf86c098504a817c800825208943535353535353535353535353535353535353535880de0b6b3a76400008025a028ef61340bd939bc2195fe537567866003e1a15d3c71ff63e1590620aa636276a067cbe9d8997f761aecb703304b3800ccf555c9f3dc64214b297fb1966a3b6d83",
  "chain_type": "ethereum",
  "derivation_path": "m/44'/60'/0'/0/0",
  "webauthn_assertion": {
    "credential_id": "base64url...",
    "authenticator_data": "base64url...",
    "client_data_json": "base64url...",
    "signature": "base64url..."
  }
}

Response: 200 OK
{
  "session_id": "uuid",
  "status": "pending",
  "websocket_url": "ws://localhost:8080/v1/ws?token=<session_token>",
  "estimated_time": "2s"
}

说明:
- 必须提供 webauthn_assertion
- 返回签名会话 ID
- Client 通过 WebSocket 参与签名协议
```

### 3.2 查询签名状态

```http
GET /v1/signing/sessions/{session_id}
Authorization: Bearer <jwt>

Response: 200 OK
{
  "session_id": "uuid",
  "wallet_id": "uuid",
  "status": "pending" | "signing" | "completed" | "failed",
  "progress": {
    "current_round": 1,
    "total_rounds": 6
  },
  "signature": "0x...",  // 完成后才有
  "created_at": "2025-01-21T10:00:00Z",
  "completed_at": "2025-01-21T10:00:02Z"
}
```

---

## 4. WebSocket 接口

### 4.1 连接

```
WS ws://localhost:8080/v1/ws?token=<session_token>
```

**认证**: Session Token (从创建钱包/签名接口返回)

### 4.2 消息格式

#### Client → Service (发送协议消息)
```json
{
  "type": "protocol_message",
  "session_id": "uuid",
  "from_node_id": "mobile-p1",
  "to_node_id": "server-signer-p2",
  "message_data": "base64...",  // tss-lib 序列化的消息
  "round": 1,
  "is_broadcast": false
}
```

#### Service → Client (接收协议消息)
```json
{
  "type": "protocol_message",
  "session_id": "uuid",
  "from_node_id": "server-signer-p2",
  "to_node_id": "mobile-p1",
  "message_data": "base64...",
  "round": 1
}
```

#### Service → Client (DKG 完成)
```json
{
  "type": "dkg_completed",
  "session_id": "uuid",
  "wallet_id": "uuid",
  "public_key": "0x...",
  "address": "0x..."
}
```

#### Service → Client (签名完成)
```json
{
  "type": "sign_completed",
  "session_id": "uuid",
  "signature": "0x..."
}
```

#### Service → Client (进度更新)
```json
{
  "type": "progress",
  "session_id": "uuid",
  "current_round": 1,
  "total_rounds": 6,
  "status": "signing"
}
```

#### Service → Client (错误)
```json
{
  "type": "error",
  "session_id": "uuid",
  "error_code": "TIMEOUT",
  "error_message": "Protocol timeout"
}
```

---

## 5. 系统接口

```http
GET /-/healthy?mgmt-secret=<secret>
Response: 200 OK
{
  "status": "healthy",
  "checks": {
    "database": "ok",
    "redis": "ok",
    "consul": "ok"
  }
}

GET /-/ready
Response: 200 OK
"Ready."

GET /-/version
Response: 200 OK
{
  "version": "0.1.0",
  "commit": "abc123",
  "build_date": "2025-01-21"
}
```

---

## 📡 gRPC Client 接口（Service → Signer）

### Proto 定义

```protobuf
// proto/mpc/v1/signer.proto
syntax = "proto3";
package mpc.v1;

option go_package = "github.com/SafeMPC/mpc-service/pb/mpc/v1;mpc";

// Signer 服务（由 mpc-signer 实现）
service SignerService {
  // DKG 相关
  rpc StartDKG(StartDKGRequest) returns (StartDKGResponse);
  rpc GetDKGStatus(GetDKGStatusRequest) returns (DKGStatusResponse);
  
  // 签名相关
  rpc StartSign(StartSignRequest) returns (StartSignResponse);
  rpc GetSignStatus(GetSignStatusRequest) returns (SignStatusResponse);
  
  // 协议消息中继
  rpc RelayProtocolMessage(RelayMessageRequest) returns (RelayMessageResponse);
  
  // 健康检查
  rpc Ping(PingRequest) returns (PongResponse);
}

// ============================================
// DKG 相关消息
// ============================================

message StartDKGRequest {
  string session_id = 1;
  string key_id = 2;
  string algorithm = 3;      // "ECDSA"
  string curve = 4;          // "secp256k1"
  int32 threshold = 5;       // 2
  int32 total_nodes = 6;     // 2
  repeated string node_ids = 7;  // ["mobile-p1", "server-signer-p2"]
}

message StartDKGResponse {
  bool started = 1;
  string message = 2;
  string error = 3;
}

message GetDKGStatusRequest {
  string session_id = 1;
}

message DKGStatusResponse {
  string session_id = 1;
  string status = 2;  // "pending", "running", "completed", "failed"
  int32 current_round = 3;
  int32 total_rounds = 4;
  string public_key = 5;  // 完成后返回
  string error = 6;
}

// ============================================
// 签名相关消息
// ============================================

message StartSignRequest {
  string session_id = 1;
  string key_id = 2;
  bytes message = 3;
  string protocol = 4;       // "gg20"
  int32 threshold = 5;
  repeated string node_ids = 6;
  string derivation_path = 7;
  bytes parent_chain_code = 8;
}

message StartSignResponse {
  bool started = 1;
  string message = 2;
  string error = 3;
}

message GetSignStatusRequest {
  string session_id = 1;
}

message SignStatusResponse {
  string session_id = 1;
  string status = 2;
  int32 current_round = 3;
  int32 total_rounds = 4;
  string signature = 5;  // 完成后返回
  string error = 6;
}

// ============================================
// 协议消息中继
// ============================================

message RelayMessageRequest {
  string session_id = 1;
  string from_node_id = 2;    // "mobile-p1"
  string to_node_id = 3;      // "server-signer-p2"
  bytes message_data = 4;     // tss-lib 序列化的消息
  int32 round = 5;
  bool is_broadcast = 6;
  string timestamp = 7;
  bytes service_signature = 8;  // Service 对消息的 HMAC 签名
}

message RelayMessageResponse {
  bool accepted = 1;
  string message_id = 2;
  // 如果 Signer 有回复消息，直接返回
  bytes reply_message = 3;
  bool has_reply = 4;
  int32 next_round = 5;
}

// ============================================
// 健康检查
// ============================================

message PingRequest {
  string from_service = 1;
}

message PongResponse {
  bool alive = 1;
  string node_id = 2;
  string timestamp = 3;
}
```

---

## 💡 gRPC Client 使用示例

### 启动 DKG

```go
// internal/infra/service/service.go
func (s *Service) CreateDKGSession(ctx context.Context, req *CreateDKGSessionRequest) (*DKGSession, error) {
  // 1. 创建会话
  session := &session.Session{
    SessionID:  generateID(),
    KeyID:      req.KeyID,
    Protocol:   "gg20",
    Threshold:  2,
    TotalNodes: 2,
    Status:     "pending",
  }
  s.sessionManager.CreateSession(ctx, session)
  
  // 2. 通过 gRPC 通知 Signer 启动 DKG
  grpcReq := &pb.StartDKGRequest{
    SessionId:  session.SessionID,
    KeyId:      req.KeyID,
    Algorithm:  req.Algorithm,
    Curve:      req.Curve,
    Threshold:  2,
    TotalNodes: 2,
    NodeIds:    []string{"mobile-p1", "server-signer-p2"},
  }
  
  resp, err := s.grpcClient.StartDKG(ctx, "server-signer-p2", grpcReq)
  if err != nil {
    return nil, err
  }
  
  // 3. 返回会话信息
  return &DKGSession{
    SessionID:  session.SessionID,
    Status:     "pending",
    WebSocketURL: fmt.Sprintf("ws://localhost:8080/v1/ws?token=%s", sessionToken),
  }, nil
}
```

### 中继协议消息

```go
// internal/infra/websocket/server.go
func (s *Server) HandleClientMessage(conn *websocket.Conn, msg *ProtocolMessage) error {
  // 1. 验证 session
  session := s.sessions[msg.SessionID]
  
  // 2. 签名消息
  signature := s.signMessage(msg.MessageData, session.SessionKey)
  
  // 3. 中继到 Signer
  grpcReq := &pb.RelayMessageRequest{
    SessionId:        msg.SessionID,
    FromNodeId:       msg.FromNodeID,  // "mobile-p1"
    ToNodeId:         msg.ToNodeID,    // "server-signer-p2"
    MessageData:      msg.MessageData,
    Round:            msg.Round,
    ServiceSignature: signature,
  }
  
  resp, err := s.grpcClient.RelayMessage(ctx, "server-signer-p2", grpcReq)
  if err != nil {
    return err
  }
  
  // 4. 如果有回复，立即发送给 Client
  if resp.HasReply {
    replyMsg := &ProtocolMessage{
      Type:        "protocol_message",
      FromNodeID:  msg.ToNodeID,
      ToNodeID:    msg.FromNodeID,
      MessageData: resp.ReplyMessage,
      Round:       resp.NextRound,
    }
    return s.sendToClient(session.UserID, replyMsg)
  }
  
  return nil
}
```

---

## 📋 实现清单

### REST API Handlers（待实现）

#### 认证
- [ ] `POST /v1/auth/webauthn/register/begin`
- [ ] `POST /v1/auth/webauthn/register/finish`
- [ ] `POST /v1/auth/webauthn/login/begin`
- [ ] `POST /v1/auth/webauthn/login/finish`

#### 钱包
- [ ] `POST /v1/wallets` (创建钱包/DKG)
- [ ] `GET /v1/wallets` (列表)
- [ ] `GET /v1/wallets/{id}` (详情)
- [ ] `POST /v1/wallets/{id}/addresses` (生成地址)

#### 签名
- [ ] `POST /v1/wallets/{id}/sign`
- [ ] `GET /v1/signing/sessions/{id}`

### WebSocket（待实现）

- [ ] WebSocket 服务器
- [ ] 会话管理
- [ ] 消息路由（Client ↔ Signer）
- [ ] 事件推送

### gRPC Client（部分已实现）

- [x] StartDKG（已有框架）
- [x] StartSign（已有框架）
- [ ] RelayProtocolMessage（新增）
- [ ] GetDKGStatus（新增）
- [ ] GetSignStatus（新增）

---

## 🎯 开发优先级

### P0 (本周)
1. WebAuthn Handlers (2小时)
2. WebSocket 服务器 (1天)
3. 消息中继逻辑 (1天)

### P1 (下周)
1. 钱包管理 Handlers (2小时)
2. 签名 Handlers (2小时)
3. gRPC Client 完善 (2小时)

### P2 (第3周)
1. 区块链集成 (Ethereum)
2. 端到端测试
3. iOS 客户端

---

**接口定义完成！可以开始实现了！** 🚀
