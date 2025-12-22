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
)

func main() {
	action := flag.String("action", "", "Action: 'keygen' or 'sign'")
	// Keygen params
	// None

	// Sign params
	privateKeyHex := flag.String("privkey", "", "Private key (hex) for signing")
	messageHex := flag.String("msg", "", "Message (hex) to sign")

	flag.Parse()

	if *action == "keygen" {
		generateKey()
	} else if *action == "sign" {
		if *privateKeyHex == "" || *messageHex == "" {
			log.Fatal("Missing -privkey or -msg for sign action")
		}
		signMessage(*privateKeyHex, *messageHex)
	} else {
		log.Fatal("Invalid action. Use 'keygen' or 'sign'")
	}
}

func generateKey() {
	// Generate ECDSA P-256 Key
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatalf("Failed to generate key: %v", err)
	}

	pubKey := privKey.PublicKey
	// Serialize Public Key to Uncompressed Hex (04 || X || Y)
	pubKeyBytes := elliptic.Marshal(elliptic.P256(), pubKey.X, pubKey.Y)
	pubKeyHex := hex.EncodeToString(pubKeyBytes)

	// Serialize Private Key (D)
	privKeyBytes := privKey.D.Bytes()
	privKeyHex := hex.EncodeToString(privKeyBytes)

	// Create a random credential ID
	credIDBytes := make([]byte, 16)
	rand.Read(credIDBytes)
	credID := base64.RawURLEncoding.EncodeToString(credIDBytes)

	result := map[string]string{
		"private_key":   privKeyHex,
		"public_key":    pubKeyHex,
		"credential_id": credID,
	}

	jsonOutput, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonOutput))
}

func signMessage(privKeyHex string, messageHex string) {
	// 1. Decode Private Key
	privBytes, err := hex.DecodeString(privKeyHex)
	if err != nil {
		log.Fatalf("Invalid private key hex: %v", err)
	}
	privKey := new(ecdsa.PrivateKey)
	privKey.PublicKey.Curve = elliptic.P256()
	privKey.D = new(big.Int).SetBytes(privBytes)
	privKey.PublicKey.X, privKey.PublicKey.Y = privKey.PublicKey.Curve.ScalarBaseMult(privBytes)

	// 2. Decode Message
	msgBytes, err := hex.DecodeString(messageHex)
	if err != nil {
		log.Fatalf("Invalid message hex: %v", err)
	}

	// 3. Construct Challenge = Base64URL(msgBytes)
	// The server code: expectedChallenge := base64.RawURLEncoding.EncodeToString(msg)
	challenge := base64.RawURLEncoding.EncodeToString(msgBytes)

	// 4. Construct ClientDataJSON
	clientData := map[string]string{
		"type":      "webauthn.get",
		"challenge": challenge,
		"origin":    "http://localhost:8080", // Example origin
	}
	clientDataJSON, _ := json.Marshal(clientData)

	// 5. Construct AuthenticatorData
	// 32 bytes RP ID Hash (sha256("localhost"))
	rpIDHash := sha256.Sum256([]byte("localhost"))
	// Flags: UP (0x01) | UV (0x04) -> let's use UP only for simplicity, or 0x05
	flags := byte(0x01)
	// Counter: 0
	counter := []byte{0, 0, 0, 0}

	authData := make([]byte, 0)
	authData = append(authData, rpIDHash[:]...)
	authData = append(authData, flags)
	authData = append(authData, counter...)
	// No attested credential data, no extensions

	// 6. Sign: sha256(authData || sha256(clientDataJSON))
	clientDataHash := sha256.Sum256(clientDataJSON)
	signedData := append(authData, clientDataHash[:]...)
	signedDataHash := sha256.Sum256(signedData)

	r, s, err := ecdsa.Sign(rand.Reader, privKey, signedDataHash[:])
	if err != nil {
		log.Fatalf("Failed to sign: %v", err)
	}

	// 7. Serialize Signature (ASN.1 DER)
	// We can use a helper or simple ASN.1 construction
	// But standard library doesn't expose easy DER encoder for (r,s) directly without import "encoding/asn1"
	// Let's use encoding/asn1
	type ECDSASignature struct {
		R, S *big.Int
	}
	signature, err := asn1.Marshal(ECDSASignature{r, s})
	if err != nil {
		log.Fatalf("Failed to marshal signature: %v", err)
	}

	// Output
	result := map[string]string{
		"authenticator_data": base64.RawStdEncoding.EncodeToString(authData),
		"client_data_json":   base64.RawStdEncoding.EncodeToString(clientDataJSON),
		"passkey_signature":  base64.RawStdEncoding.EncodeToString(signature),
		"credential_id":      "YOUR_CREDENTIAL_ID", // Placeholder, user should know this from keygen
	}

	jsonOutput, _ := json.MarshalIndent(result, "", "  ")
	fmt.Println(string(jsonOutput))
}
