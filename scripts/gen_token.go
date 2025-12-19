package main

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func main() {
	secretKey := []byte("change-me-in-production")
	issuer := "mpc-infra"
	
	claims := struct {
		jwt.RegisteredClaims
		TenantID    string   `json:"tenant_id,omitempty"`
		Permissions []string `json:"permissions,omitempty"`
		AppID       string   `json:"app_id,omitempty"`
	}{
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    issuer,
			Subject:   "system-test",
		},
		TenantID:    "test-tenant",
		Permissions: []string{"admin"},
		AppID:       "system-test",
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signedToken, err := token.SignedString(secretKey)
	if err != nil {
		panic(err)
	}
	fmt.Println(signedToken)
}
