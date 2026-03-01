package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppPort           string
	AppBaseURL        string
	FrontendOrigin    string
	DefaultAdminUser  string
	DefaultAdminPass  string
	DefaultAdminName  string
	PostgresDSN       string
	JWTSecret         string
	AccessTokenTTL    time.Duration
	RefreshTokenTTL   time.Duration
	MinioEndpoint     string
	MinioAccessKey    string
	MinioSecretKey    string
	MinioBucket       string
	MinioUseSSL       bool
	UploadURLExpire   time.Duration
	DownloadURLExpire time.Duration
	MaxUploadSizeMB   int64
}

func Load() (Config, error) {
	cfg := Config{
		AppPort:           getEnv("APP_PORT", "8088"),
		AppBaseURL:        getEnv("APP_BASE_URL", "http://localhost:8088"),
		FrontendOrigin:    getEnv("FRONTEND_ORIGIN", "http://localhost:5173"),
		DefaultAdminUser:  getEnv("DEFAULT_ADMIN_USERNAME", "admin"),
		DefaultAdminPass:  getEnv("DEFAULT_ADMIN_PASSWORD", "12345678"),
		DefaultAdminName:  getEnv("DEFAULT_ADMIN_DISPLAY_NAME", "系统管理员"),
		PostgresDSN:       getEnv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:55432/alliance_vault?sslmode=disable"),
		JWTSecret:         getEnv("JWT_SECRET", "alliance-vault-dev-secret"),
		AccessTokenTTL:    time.Duration(parseInt(getEnv("ACCESS_TOKEN_TTL_MINUTES", "30"), 30)) * time.Minute,
		RefreshTokenTTL:   time.Duration(parseInt(getEnv("REFRESH_TOKEN_TTL_HOURS", "168"), 168)) * time.Hour,
		MinioEndpoint:     getEnv("MINIO_ENDPOINT", "localhost:9000"),
		MinioAccessKey:    getEnv("MINIO_ACCESS_KEY", "rustfsadmin"),
		MinioSecretKey:    getEnv("MINIO_SECRET_KEY", "rustfsadmin"),
		MinioBucket:       getEnv("MINIO_BUCKET", "alliance-vault"),
		MinioUseSSL:       parseBool(getEnv("MINIO_USE_SSL", "false")),
		UploadURLExpire:   time.Duration(parseInt(getEnv("UPLOAD_URL_EXPIRE_SECONDS", "900"), 900)) * time.Second,
		DownloadURLExpire: time.Duration(parseInt(getEnv("DOWNLOAD_URL_EXPIRE_SECONDS", "600"), 600)) * time.Second,
		MaxUploadSizeMB:   parseInt64(getEnv("MAX_UPLOAD_SIZE_MB", "20"), 20),
	}

	if strings.TrimSpace(cfg.PostgresDSN) == "" {
		return Config{}, fmt.Errorf("POSTGRES_DSN is required")
	}
	if strings.TrimSpace(cfg.DefaultAdminUser) == "" {
		return Config{}, fmt.Errorf("DEFAULT_ADMIN_USERNAME is required")
	}
	if strings.TrimSpace(cfg.DefaultAdminPass) == "" {
		return Config{}, fmt.Errorf("DEFAULT_ADMIN_PASSWORD is required")
	}
	if strings.TrimSpace(cfg.JWTSecret) == "" {
		return Config{}, fmt.Errorf("JWT_SECRET is required")
	}
	if strings.TrimSpace(cfg.MinioBucket) == "" {
		return Config{}, fmt.Errorf("MINIO_BUCKET is required")
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return strings.TrimSpace(v)
	}
	return fallback
}

func parseBool(raw string) bool {
	v, err := strconv.ParseBool(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	return v
}

func parseInt(raw string, fallback int) int {
	v, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return fallback
	}
	return v
}

func parseInt64(raw string, fallback int64) int64 {
	v, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
	if err != nil {
		return fallback
	}
	return v
}
