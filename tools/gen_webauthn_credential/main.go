package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/asn1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"

	"github.com/fxamacker/cbor/v2"
)

func main() {
	action := flag.String("action", "", "Action: 'register' or 'login'")
	challenge := flag.String("challenge", "", "Challenge from server (base64url)")
	credentialID := flag.String("credential-id", "", "Credential ID (for login, base64url)")
	privateKeyHex := flag.String("privkey", "", "Private key (hex) - if not provided, will generate new one")
	origin := flag.String("origin", "http://localhost:8080", "Origin URL")
	rpID := flag.String("rp-id", "localhost", "Relying Party ID")

	flag.Parse()

	if *action == "register" {
		if *challenge == "" {
			log.Fatal("challenge is required for register action")
		}
		generateRegistrationResponse(*challenge, *privateKeyHex, *origin, *rpID)
	} else if *action == "login" {
		if *challenge == "" || *credentialID == "" || *privateKeyHex == "" {
			log.Fatal("challenge, credential-id, and privkey are required for login action")
		}
		generateLoginResponse(*challenge, *credentialID, *privateKeyHex, *origin, *rpID)
	} else {
		log.Fatal("Invalid action. Use 'register' or 'login'")
	}
}

// generateRegistrationResponse 生成 WebAuthn 注册响应
func generateRegistrationResponse(challengeBase64, privateKeyHex, origin, rpID string) {
	// 1. 验证 challenge 格式（但不使用解码后的字节）
	_, err := base64.RawURLEncoding.DecodeString(challengeBase64)
	if err != nil {
		log.Fatalf("Failed to decode challenge: %v", err)
	}

	// 2. 生成或使用提供的私钥
	var privKey *ecdsa.PrivateKey
	var pubKeyBytes []byte
	var credentialIDBytes []byte

	if privateKeyHex == "" {
		// 生成新密钥
		privKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			log.Fatalf("Failed to generate key: %v", err)
		}
		pubKeyBytes = elliptic.Marshal(elliptic.P256(), privKey.PublicKey.X, privKey.PublicKey.Y)

		// 生成 credential ID
		credentialIDBytes = make([]byte, 16)
		rand.Read(credentialIDBytes)
	} else {
		// 使用提供的私钥
		privBytes, err := hex.DecodeString(privateKeyHex)
		if err != nil {
			log.Fatalf("Failed to decode private key: %v", err)
		}
		privKey = new(ecdsa.PrivateKey)
		privKey.PublicKey.Curve = elliptic.P256()
		privKey.D = new(big.Int).SetBytes(privBytes)
		privKey.PublicKey.X, privKey.PublicKey.Y = privKey.PublicKey.Curve.ScalarBaseMult(privBytes)
		pubKeyBytes = elliptic.Marshal(elliptic.P256(), privKey.PublicKey.X, privKey.PublicKey.Y)

		// 从私钥生成 credential ID（确定性）
		credIDHash := sha256.Sum256(privBytes)
		credentialIDBytes = credIDHash[:16]
	}

	credentialIDBase64 := base64.RawURLEncoding.EncodeToString(credentialIDBytes)

	// 3. 构建 ClientDataJSON
	clientData := map[string]interface{}{
		"type":      "webauthn.create",
		"challenge": challengeBase64,
		"origin":    origin,
	}
	clientDataJSON, err := json.Marshal(clientData)
	if err != nil {
		log.Fatalf("Failed to marshal client data: %v", err)
	}
	clientDataJSONBase64 := base64.RawURLEncoding.EncodeToString(clientDataJSON)

	// 4. 构建 AuthenticatorData
	rpIDHash := sha256.Sum256([]byte(rpID))
	flags := byte(0x41) // UP (0x01) | AT (0x40) - User Present + Attested Credential Data
	counter := []byte{0, 0, 0, 0}

	// Attested Credential Data
	// AAGUID (16 bytes of zeros for no attestation)
	aaguid := make([]byte, 16)
	// Credential ID Length (2 bytes, big-endian)
	credIDLen := []byte{byte(len(credentialIDBytes) >> 8), byte(len(credentialIDBytes) & 0xff)}
	// Credential Public Key (CBOR encoded COSE_Key)
	coseKey := buildCOSEPublicKey(pubKeyBytes)
	coseKeyCBOR, err := cbor.Marshal(coseKey)
	if err != nil {
		log.Fatalf("Failed to marshal COSE key: %v", err)
	}

	// AuthenticatorData = RP ID Hash (32) + Flags (1) + Counter (4) + Attested Credential Data
	authData := make([]byte, 0, 37+16+2+len(credentialIDBytes)+len(coseKeyCBOR))
	authData = append(authData, rpIDHash[:]...)
	authData = append(authData, flags)
	authData = append(authData, counter...)
	authData = append(authData, aaguid...)
	authData = append(authData, credIDLen...)
	authData = append(authData, credentialIDBytes...)
	authData = append(authData, coseKeyCBOR...)

	// 5. 构建 Attestation Object (CBOR)
	// 注意：对于 "none" attestation，我们不需要签名
	attestationObject := map[string]interface{}{
		"authData": authData,
		"fmt":      "none", // No attestation
		"attStmt":  map[string]interface{}{}, // Empty attestation statement
	}
	attestationObjectCBOR, err := cbor.Marshal(attestationObject)
	if err != nil {
		log.Fatalf("Failed to marshal attestation object: %v", err)
	}

	// 7. 构建完整的响应
	response := map[string]interface{}{
		"id":       credentialIDBase64,
		"rawId":    base64.RawURLEncoding.EncodeToString(credentialIDBytes),
		"type":     "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSONBase64,
			"attestationObject": base64.RawURLEncoding.EncodeToString(attestationObjectCBOR),
		},
	}

	output, _ := json.MarshalIndent(response, "", "  ")
	fmt.Println(string(output))

	// 输出私钥和 credential ID 供后续使用
	fmt.Fprintf(log.Writer(), "\n=== Save these for login ===\n")
	fmt.Fprintf(log.Writer(), "Credential ID: %s\n", credentialIDBase64)
	privKeyBytes := privKey.D.Bytes()
	fmt.Fprintf(log.Writer(), "Private Key: %s\n", hex.EncodeToString(privKeyBytes))
}

// generateLoginResponse 生成 WebAuthn 登录响应
func generateLoginResponse(challengeBase64, credentialIDBase64, privateKeyHex, origin, rpID string) {
	// 1. 验证 challenge 格式（但不使用解码后的字节）
	_, err := base64.RawURLEncoding.DecodeString(challengeBase64)
	if err != nil {
		log.Fatalf("Failed to decode challenge: %v", err)
	}

	// 2. 解析 credential ID
	credentialIDBytes, err := base64.RawURLEncoding.DecodeString(credentialIDBase64)
	if err != nil {
		log.Fatalf("Failed to decode credential ID: %v", err)
	}

	// 3. 解析私钥
	privBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		log.Fatalf("Failed to decode private key: %v", err)
	}
	privKey := new(ecdsa.PrivateKey)
	privKey.PublicKey.Curve = elliptic.P256()
	privKey.D = new(big.Int).SetBytes(privBytes)
	privKey.PublicKey.X, privKey.PublicKey.Y = privKey.PublicKey.Curve.ScalarBaseMult(privBytes)

	// 4. 构建 ClientDataJSON
	clientData := map[string]interface{}{
		"type":      "webauthn.get",
		"challenge": challengeBase64,
		"origin":    origin,
	}
	clientDataJSON, err := json.Marshal(clientData)
	if err != nil {
		log.Fatalf("Failed to marshal client data: %v", err)
	}
	clientDataJSONBase64 := base64.RawURLEncoding.EncodeToString(clientDataJSON)

	// 5. 构建 AuthenticatorData
	rpIDHash := sha256.Sum256([]byte(rpID))
	flags := byte(0x01) // UP (User Present)
	counter := []byte{0, 0, 0, 1} // Increment counter

	authData := make([]byte, 0, 37)
	authData = append(authData, rpIDHash[:]...)
	authData = append(authData, flags)
	authData = append(authData, counter...)

	// 6. 构建签名
	clientDataHash := sha256.Sum256(clientDataJSON)
	signedData := append(authData, clientDataHash[:]...)
	signedDataHash := sha256.Sum256(signedData)

	r, s, err := ecdsa.Sign(rand.Reader, privKey, signedDataHash[:])
	if err != nil {
		log.Fatalf("Failed to sign: %v", err)
	}

	type ECDSASignature struct {
		R, S *big.Int
	}
	signature, err := asn1.Marshal(ECDSASignature{r, s})
	if err != nil {
		log.Fatalf("Failed to marshal signature: %v", err)
	}

	// 7. 构建完整的响应
	response := map[string]interface{}{
		"id":       credentialIDBase64,
		"rawId":    base64.RawURLEncoding.EncodeToString(credentialIDBytes),
		"type":     "public-key",
		"response": map[string]interface{}{
			"clientDataJSON":    clientDataJSONBase64,
			"authenticatorData": base64.RawURLEncoding.EncodeToString(authData),
			"signature":          base64.RawURLEncoding.EncodeToString(signature),
			"userHandle":         nil,
		},
	}

	output, _ := json.MarshalIndent(response, "", "  ")
	fmt.Println(string(output))
}

// buildCOSEPublicKey 构建 COSE 格式的公钥
func buildCOSEPublicKey(pubKeyBytes []byte) map[int]interface{} {
	// COSE_Key format for ECDSA P-256
	// Key type: 2 (EC2)
	// Algorithm: -7 (ES256)
	// Curve: 1 (P-256)
	// X coordinate: bytes 1-32 of pubKeyBytes (skip 0x04 prefix)
	// Y coordinate: bytes 33-64 of pubKeyBytes

	if len(pubKeyBytes) != 65 || pubKeyBytes[0] != 0x04 {
		log.Fatalf("Invalid public key format: expected 65 bytes starting with 0x04, got %d bytes", len(pubKeyBytes))
	}

	x := pubKeyBytes[1:33]
	y := pubKeyBytes[33:65]

	return map[int]interface{}{
		1:  2,        // kty: EC2
		3:  -7,       // alg: ES256
		-1: 1,        // crv: P-256
		-2: x,        // x coordinate
		-3: y,        // y coordinate
	}
}
