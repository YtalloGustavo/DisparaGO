package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"

	"disparago/internal/config"
	authdomain "disparago/internal/domain/auth"
	"disparago/internal/repository"
)

var (
	ErrInvalidCredentials = errors.New("credenciais invalidas")
	ErrInvalidToken       = errors.New("token invalido")
	ErrInactiveUser       = errors.New("usuario inativo")
)

type AuthService struct {
	repository *repository.AuthRepository
	cfg        config.AuthConfig
	secret     []byte
	tokenTTL   time.Duration
}

type AuthClaims struct {
	UserID      int64           `json:"user_id"`
	CompanyID   int64           `json:"company_id"`
	CompanyName string          `json:"company_name"`
	Username    string          `json:"username"`
	DisplayName string          `json:"display_name"`
	Role        authdomain.Role `json:"role"`
	IssuedAt    time.Time       `json:"issued_at"`
	ExpiresAt   time.Time       `json:"expires_at"`
}

type SyncCompanyInput struct {
	Name           string
	ExternalSource string
	ExternalID     string
}

type SyncUserInput struct {
	CompanyID      *int64
	Username       string
	DisplayName    string
	Password       string
	Role           authdomain.Role
	Active         bool
	ExternalSource string
	ExternalID     string
}

func NewAuthService(repository *repository.AuthRepository, cfg config.AuthConfig) *AuthService {
	return &AuthService{
		repository: repository,
		cfg:        cfg,
		secret:     []byte(cfg.Secret),
		tokenTTL:   cfg.TokenTTL,
	}
}

func (s *AuthService) EnsureBootstrap(ctx context.Context) error {
	company, err := s.UpsertCompany(ctx, SyncCompanyInput{
		Name:           s.cfg.BootstrapCompanyName,
		ExternalSource: "bootstrap",
		ExternalID:     "default",
	})
	if err != nil {
		return fmt.Errorf("bootstrap company: %w", err)
	}

	if _, err := s.UpsertUser(ctx, SyncUserInput{
		CompanyID:   &company.ID,
		Username:    s.cfg.OperatorUsername,
		DisplayName: s.cfg.OperatorDisplayName,
		Password:    s.cfg.OperatorPassword,
		Role:        authdomain.RoleOperator,
		Active:      true,
	}); err != nil {
		return fmt.Errorf("bootstrap operator: %w", err)
	}

	if _, err := s.UpsertUser(ctx, SyncUserInput{
		Username:    s.cfg.SuperadminUsername,
		DisplayName: s.cfg.SuperadminDisplayName,
		Password:    s.cfg.SuperadminPassword,
		Role:        authdomain.RoleSuperadmin,
		Active:      true,
	}); err != nil {
		return fmt.Errorf("bootstrap superadmin: %w", err)
	}

	return nil
}

func (s *AuthService) Login(ctx context.Context, username, password string) (string, AuthClaims, error) {
	username = strings.TrimSpace(username)
	user, err := s.repository.FindUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, repository.ErrAuthUserNotFound) {
			return "", AuthClaims{}, ErrInvalidCredentials
		}
		return "", AuthClaims{}, err
	}

	if !user.Active {
		return "", AuthClaims{}, ErrInactiveUser
	}

	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return "", AuthClaims{}, ErrInvalidCredentials
	}

	claims, err := s.userClaims(ctx, user)
	if err != nil {
		return "", AuthClaims{}, err
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

	if claims.UserID == 0 || claims.Username == "" || claims.Role == "" || time.Now().UTC().After(claims.ExpiresAt) {
		return AuthClaims{}, ErrInvalidToken
	}

	return claims, nil
}

func (s *AuthService) UpsertCompany(ctx context.Context, input SyncCompanyInput) (authdomain.Company, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return authdomain.Company{}, fmt.Errorf("%w: company name is required", ErrInvalidCredentials)
	}

	return s.repository.UpsertCompany(ctx, repository.UpsertCompanyParams{
		Name:           name,
		ExternalSource: strings.TrimSpace(input.ExternalSource),
		ExternalID:     strings.TrimSpace(input.ExternalID),
	})
}

func (s *AuthService) UpsertUser(ctx context.Context, input SyncUserInput) (authdomain.User, error) {
	username := strings.TrimSpace(input.Username)
	displayName := strings.TrimSpace(input.DisplayName)
	if username == "" {
		return authdomain.User{}, fmt.Errorf("%w: username is required", ErrInvalidCredentials)
	}
	if displayName == "" {
		displayName = username
	}

	passwordHash, err := bcrypt.GenerateFromPassword([]byte(input.Password), bcrypt.DefaultCost)
	if err != nil {
		return authdomain.User{}, fmt.Errorf("hash password: %w", err)
	}

	return s.repository.UpsertUser(ctx, repository.UpsertUserParams{
		CompanyID:      input.CompanyID,
		Username:       username,
		DisplayName:    displayName,
		PasswordHash:   string(passwordHash),
		Role:           input.Role,
		Active:         input.Active,
		ExternalSource: strings.TrimSpace(input.ExternalSource),
		ExternalID:     strings.TrimSpace(input.ExternalID),
	})
}

func (s *AuthService) userClaims(ctx context.Context, user authdomain.User) (AuthClaims, error) {
	now := time.Now().UTC()
	claims := AuthClaims{
		UserID:      user.ID,
		Username:    user.Username,
		DisplayName: user.DisplayName,
		Role:        user.Role,
		IssuedAt:    now,
		ExpiresAt:   now.Add(s.tokenTTL),
	}

	if user.CompanyID != nil {
		claims.CompanyID = *user.CompanyID
		company, err := s.repository.FindCompanyByID(ctx, *user.CompanyID)
		if err != nil {
			return AuthClaims{}, fmt.Errorf("load company: %w", err)
		}
		claims.CompanyName = company.Name
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
