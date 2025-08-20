package gateway

import (
	"crypto/rand"
	"crypto/rsa"
	"testing"
	"time"
)

func TestJWTGenerateAndParse(t *testing.T) {
	gw := &Gateway{}
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	gw.signingKey = key
	gw.keyID = "kid"

	tok, exp, err := gw.generateJWT("ns1", "subj", time.Minute)
	if err != nil || exp <= 0 {
		t.Fatalf("gen err=%v exp=%d", err, exp)
	}

	claims, err := gw.parseAndVerifyJWT(tok)
	if err != nil {
		t.Fatalf("verify err: %v", err)
	}
	if claims.Namespace != "ns1" || claims.Sub != "subj" || claims.Aud != "gateway" || claims.Iss != "debros-gateway" {
		t.Fatalf("unexpected claims: %+v", claims)
	}
}

func TestJWTExpired(t *testing.T) {
	gw := &Gateway{}
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	gw.signingKey = key
	gw.keyID = "kid"

	// Use sufficiently negative TTL to bypass allowed clock skew
	tok, _, err := gw.generateJWT("ns1", "subj", -2*time.Minute)
	if err != nil {
		t.Fatalf("gen err=%v", err)
	}
	if _, err := gw.parseAndVerifyJWT(tok); err == nil {
		t.Fatalf("expected expired error")
	}
}
