package rest

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/KilimcininKorOglu/oba/internal/backend"
)

// Auth errors.
var (
	ErrInvalidToken = errors.New("rest: invalid token")
	ErrTokenExpired = errors.New("rest: token expired")
)

// JWTClaims represents JWT claims.
type JWTClaims struct {
	DN        string `json:"dn"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// Authenticator handles authentication.
type Authenticator struct {
	backend   *backend.ObaBackend
	jwtSecret []byte
	tokenTTL  time.Duration
	mu        sync.RWMutex
}

// NewAuthenticator creates a new authenticator.
func NewAuthenticator(be *backend.ObaBackend, jwtSecret string, tokenTTL time.Duration) *Authenticator {
	return &Authenticator{
		backend:   be,
		jwtSecret: []byte(jwtSecret),
		tokenTTL:  tokenTTL,
	}
}

// SetTokenTTL updates the token TTL at runtime.
func (a *Authenticator) SetTokenTTL(ttl time.Duration) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.tokenTTL = ttl
}

// GetTokenTTL returns the current token TTL.
func (a *Authenticator) GetTokenTTL() time.Duration {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.tokenTTL
}

// Authenticate validates credentials and returns a JWT token.
func (a *Authenticator) Authenticate(dn, password string) (string, error) {
	if err := a.backend.Bind(dn, password); err != nil {
		return "", err
	}

	return a.generateToken(dn)
}

// generateToken creates a JWT token for the given DN.
func (a *Authenticator) generateToken(dn string) (string, error) {
	now := time.Now()
	ttl := a.GetTokenTTL()
	claims := JWTClaims{
		DN:        dn,
		IssuedAt:  now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}

	header := map[string]string{
		"alg": "HS256",
		"typ": "JWT",
	}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	payloadJSON, _ := json.Marshal(claims)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payloadJSON)

	message := headerB64 + "." + payloadB64
	signature := a.sign([]byte(message))
	signatureB64 := base64.RawURLEncoding.EncodeToString(signature)

	return message + "." + signatureB64, nil
}

// ValidateToken validates a JWT token and returns the claims.
func (a *Authenticator) ValidateToken(token string) (*JWTClaims, error) {
	parts := splitToken(token)
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	message := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}

	expectedSig := a.sign([]byte(message))
	if !hmac.Equal(signature, expectedSig) {
		return nil, ErrInvalidToken
	}

	payloadJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims JWTClaims
	if err := json.Unmarshal(payloadJSON, &claims); err != nil {
		return nil, ErrInvalidToken
	}

	if time.Now().Unix() > claims.ExpiresAt {
		return nil, ErrTokenExpired
	}

	return &claims, nil
}

func (a *Authenticator) sign(message []byte) []byte {
	h := hmac.New(sha256.New, a.jwtSecret)
	h.Write(message)
	return h.Sum(nil)
}

func splitToken(token string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(token); i++ {
		if token[i] == '.' {
			parts = append(parts, token[start:i])
			start = i + 1
		}
	}
	parts = append(parts, token[start:])
	return parts
}
