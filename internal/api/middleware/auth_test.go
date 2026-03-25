package middleware_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/evan/football-picks/internal/api/middleware"
	"github.com/golang-jwt/jwt/v5"
)

func TestNewJWKS_InvalidURL(t *testing.T) {
	// httptest server that returns an empty JWKS — just tests the fetch path.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"keys":[]}`))
	}))
	defer srv.Close()

	k, err := middleware.NewJWKS(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("NewJWKS: %v", err)
	}
	if k == nil {
		t.Error("expected non-nil keyfunc")
	}
}

func TestSupabaseUIDFromContext_Present(t *testing.T) {
	ctx := context.WithValue(context.Background(), middleware.ContextKeySupabaseUID, "uid-abc")
	if got := middleware.SupabaseUIDFromContext(ctx); got != "uid-abc" {
		t.Errorf("got %q, want uid-abc", got)
	}
}

func TestSupabaseUIDFromContext_Missing(t *testing.T) {
	if got := middleware.SupabaseUIDFromContext(context.Background()); got != "" {
		t.Errorf("got %q, want empty string", got)
	}
}

func TestAuth_MissingToken(t *testing.T) {
	// nil keyfunc is safe here — missing header is rejected before parse
	called := false
	handler := middleware.Auth(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
	if called {
		t.Error("next handler should not be called when token is missing")
	}
}

func TestAuth_InvalidBearerToken(t *testing.T) {
	// empty key set — JWT parse will fail and return 401
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"keys":[]}`))
	}))
	defer srv.Close()

	k, err := middleware.NewJWKS(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("NewJWKS: %v", err)
	}

	handler := middleware.Auth(k)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer not-a-real-jwt")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status: got %d, want 401", rec.Code)
	}
}

// makeTestJWKSServer creates an in-process JWKS server and returns it along
// with a signed ES256 JWT containing the given subject claim.
func makeTestJWKSServer(t *testing.T, sub string) (*httptest.Server, string) {
	t.Helper()

	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	const kid = "test-key-1"

	// Encode the public key coordinates as base64url (no padding).
	byteLen := (privKey.PublicKey.Curve.Params().BitSize + 7) / 8
	xBytes := make([]byte, byteLen)
	yBytes := make([]byte, byteLen)
	privKey.PublicKey.X.FillBytes(xBytes)
	privKey.PublicKey.Y.FillBytes(yBytes)
	enc := base64.RawURLEncoding.EncodeToString

	jwksJSON, _ := json.Marshal(map[string]interface{}{
		"keys": []map[string]string{{
			"kty": "EC",
			"crv": "P-256",
			"kid": kid,
			"x":   enc(xBytes),
			"y":   enc(yBytes),
		}},
	})

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(jwksJSON)
	}))

	tok := jwt.NewWithClaims(jwt.SigningMethodES256, jwt.MapClaims{
		"sub": sub,
		"exp": time.Now().Add(time.Hour).Unix(),
	})
	tok.Header["kid"] = kid
	signed, err := tok.SignedString(privKey)
	if err != nil {
		srv.Close()
		t.Fatalf("sign JWT: %v", err)
	}

	return srv, signed
}

func TestAuth_ValidToken(t *testing.T) {
	const wantUID = "test-uid-valid"
	srv, token := makeTestJWKSServer(t, wantUID)
	defer srv.Close()

	k, err := middleware.NewJWKS(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("NewJWKS: %v", err)
	}

	var gotUID string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUID = middleware.SupabaseUIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})
	handler := middleware.Auth(k)(next)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status: got %d, want 200", rec.Code)
	}
	if gotUID != wantUID {
		t.Errorf("UID in context: got %q, want %q", gotUID, wantUID)
	}
}

func TestAuth_MalformedAuthorizationHeader(t *testing.T) {
	// wrong/missing scheme — extractBearerToken returns "" before jwks is called
	handler := middleware.Auth(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	for _, header := range []string{"Basic dXNlcjpwYXNz", "token-no-scheme"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", header)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Errorf("header %q: status got %d, want 401", header, rec.Code)
		}
	}
}
