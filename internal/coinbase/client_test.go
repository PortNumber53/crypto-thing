package coinbase

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (fn roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return fn(request)
}

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

func TestBearerTokenSupportsCoinbaseEd25519Key(t *testing.T) {
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	client, err := NewClientWithJWT("organizations/test/apiKeys/ed25519", base64.StdEncoding.EncodeToString(privateKey))
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
	headerJSON, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		t.Fatal(err)
	}
	var header map[string]any
	if err := json.Unmarshal(headerJSON, &header); err != nil {
		t.Fatal(err)
	}
	if header["alg"] != "EdDSA" || header["nonce"] == "" {
		t.Fatalf("unexpected JWT header: alg=%v nonce-present=%v", header["alg"], header["nonce"] != "")
	}
}

func TestGetProductsUsesCoinbaseMaximumLimit(t *testing.T) {
	client := NewClient("", "", "")
	client.httpClient = &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if got := request.URL.Query().Get("limit"); got != "1000" {
			t.Fatalf("limit = %q, want 1000", got)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"products":[],"num_products":0}`)),
		}, nil
	})}
	products, err := client.GetProducts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(products) != 0 {
		t.Fatalf("products = %d, want 0", len(products))
	}
}
