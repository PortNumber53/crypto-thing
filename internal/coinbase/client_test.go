package coinbase

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"strings"
	"testing"
)

func TestBearerTokenUsesCurrentCoinbaseClaims(t *testing.T) {
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalECPrivateKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyName := "organizations/test/apiKeys/test"
	client, err := NewClientWithJWT(keyName, string(pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: der})))
	if err != nil {
		t.Fatal(err)
	}

	token, err := client.bearerToken("GET", "/api/v3/brokerage/accounts")
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("JWT has %d parts, want 3", len(parts))
	}
	decode := func(part string) map[string]any {
		t.Helper()
		data, err := base64.RawURLEncoding.DecodeString(part)
		if err != nil {
			t.Fatal(err)
		}
		var value map[string]any
		if err := json.Unmarshal(data, &value); err != nil {
			t.Fatal(err)
		}
		return value
	}

	header := decode(parts[0])
	claims := decode(parts[1])
	if header["alg"] != "ES256" || header["kid"] != keyName || header["nonce"] == "" {
		t.Fatalf("unexpected JWT header: alg=%v kid=%v nonce-present=%v", header["alg"], header["kid"], header["nonce"] != "")
	}
	if claims["iss"] != "coinbase-cloud" {
		t.Fatalf("issuer = %v, want coinbase-cloud", claims["iss"])
	}
	if claims["sub"] != keyName {
		t.Fatalf("subject = %v, want key name", claims["sub"])
	}
	if claims["uri"] != "GET api.coinbase.com/api/v3/brokerage/accounts" {
		t.Fatalf("uri = %v", claims["uri"])
	}
}
