package service

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"disparago/internal/config"
)

var (
	ErrInvalidCredentials = errors.New("credenciais invalidas")
	ErrInvalidToken       = errors.New("token invalido")
)

type AuthService struct {
	users    []authUser
	secret   []byte
	tokenTTL time.Duration
}

type AuthClaims struct {
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	IssuedAt  time.Time `json:"issued_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

type authUser struct {
	Username string
	Password string
	Role     string
}

func NewAuthService(cfg config.AuthConfig) *AuthService {
	return &AuthService{
		users: []authUser{
			{
				Username: cfg.OperatorUsername,
				Password: cfg.OperatorPassword,
				Role:     "operator",
			},
			{
				Username: cfg.SuperadminUsername,
				Password: cfg.SuperadminPassword,
				Role:     "superadmin",
			},
		},
		secret:   []byte(cfg.Secret),
		tokenTTL: cfg.TokenTTL,
	}
}

func (s *AuthService) Login(username, password string) (string, AuthClaims, error) {
	username = strings.TrimSpace(username)

	var matched *authUser
	for i := range s.users {
		user := &s.users[i]
		if subtle.ConstantTimeCompare([]byte(username), []byte(user.Username)) == 1 &&
			subtle.ConstantTimeCompare([]byte(password), []byte(user.Password)) == 1 {
			matched = user
			break
		}
	}

	if matched == nil {
		return "", AuthClaims{}, ErrInvalidCredentials
	}

	now := time.Now().UTC()
	claims := AuthClaims{
		Username:  matched.Username,
		Role:      matched.Role,
		IssuedAt:  now,
		ExpiresAt: now.Add(s.tokenTTL),
	}

	token, err := s.sign(claims)
	if err != nil {
		return "", AuthClaims{}, fmt.Errorf("sign token: %w", err)
	}

	return token, claims, nil
}

func (s *AuthService) Validate(token string) (AuthClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 2 {
		return AuthClaims{}, ErrInvalidToken
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return AuthClaims{}, ErrInvalidToken
	}

	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return AuthClaims{}, ErrInvalidToken
	}

	expected := s.signature(parts[0])
	if !hmac.Equal(signature, expected) {
		return AuthClaims{}, ErrInvalidToken
	}

	var claims AuthClaims
	if err := json.Unmarshal(payloadBytes, &claims); err != nil {
		return AuthClaims{}, ErrInvalidToken
	}

	if claims.Username == "" || claims.Role == "" || time.Now().UTC().After(claims.ExpiresAt) {
		return AuthClaims{}, ErrInvalidToken
	}

	return claims, nil
}

func (s *AuthService) sign(claims AuthClaims) (string, error) {
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	encodedPayload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signature := base64.RawURLEncoding.EncodeToString(s.signature(encodedPayload))
	return encodedPayload + "." + signature, nil
}

func (s *AuthService) signature(payload string) []byte {
	mac := hmac.New(sha256.New, s.secret)
	_, _ = mac.Write([]byte(payload))
	return mac.Sum(nil)
}
