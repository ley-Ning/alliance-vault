package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	TokenTypeAccess  = "access"
	TokenTypeRefresh = "refresh"
)

type Claims struct {
	Username           string `json:"username"`
	TokenType          string `json:"tokenType"`
	IsAdmin            bool   `json:"isAdmin"`
	MustChangePassword bool   `json:"mustChangePassword"`
	jwt.RegisteredClaims
}

type TokenPair struct {
	AccessToken           string
	RefreshToken          string
	RefreshTokenID        string
	RefreshTokenExpiresAt time.Time
}

type TokenManager struct {
	secret     []byte
	issuer     string
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewTokenManager(secret, issuer string, accessTTL, refreshTTL time.Duration) (*TokenManager, error) {
	if secret == "" {
		return nil, errors.New("jwt secret is required")
	}
	if accessTTL <= 0 || refreshTTL <= 0 {
		return nil, errors.New("token ttl must be positive")
	}

	return &TokenManager{
		secret:     []byte(secret),
		issuer:     issuer,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}, nil
}

func (m *TokenManager) GenerateTokenPair(
	userID string,
	username string,
	isAdmin bool,
	mustChangePassword bool,
	now time.Time,
) (TokenPair, error) {
	accessClaims := Claims{
		Username:           username,
		TokenType:          TokenTypeAccess,
		IsAdmin:            isAdmin,
		MustChangePassword: mustChangePassword,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.accessTTL)),
		},
	}
	accessToken, err := m.sign(accessClaims)
	if err != nil {
		return TokenPair{}, err
	}

	refreshID := uuid.NewString()
	refreshExpires := now.Add(m.refreshTTL)
	refreshClaims := Claims{
		Username:           username,
		TokenType:          TokenTypeRefresh,
		IsAdmin:            isAdmin,
		MustChangePassword: mustChangePassword,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			ID:        refreshID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(refreshExpires),
		},
	}
	refreshToken, err := m.sign(refreshClaims)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:           accessToken,
		RefreshToken:          refreshToken,
		RefreshTokenID:        refreshID,
		RefreshTokenExpiresAt: refreshExpires,
	}, nil
}

func (m *TokenManager) ParseToken(tokenString string) (Claims, error) {
	parsed, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		method, ok := token.Method.(*jwt.SigningMethodHMAC)
		if !ok || method.Name != jwt.SigningMethodHS256.Name {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return Claims{}, err
	}

	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return Claims{}, errors.New("invalid token")
	}

	return *claims, nil
}

func (m *TokenManager) sign(claims Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", fmt.Errorf("sign token: %w", err)
	}
	return signed, nil
}
