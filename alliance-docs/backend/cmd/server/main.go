package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"alliance-vault/backend/internal/auth"
	"alliance-vault/backend/internal/config"
	"alliance-vault/backend/internal/httpapi"
	"alliance-vault/backend/internal/storage"
	"alliance-vault/backend/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	db, err := store.OpenPostgres(cfg.PostgresDSN)
	if err != nil {
		log.Fatalf("connect postgres failed: %v", err)
	}
	defer db.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := store.InitSchema(ctx, db); err != nil {
		cancel()
		log.Fatalf("init schema failed: %v", err)
	}
	cancel()

	userRepo := store.NewUserRepo(db)
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Second)
	defaultAdmin, created, err := userRepo.EnsureDefaultAdmin(
		ctx,
		cfg.DefaultAdminUser,
		cfg.DefaultAdminPass,
		cfg.DefaultAdminName,
	)
	cancel()
	if err != nil {
		log.Fatalf("ensure default admin failed: %v", err)
	}
	if created {
		log.Printf(
			"default admin user initialized: username=%s, password=%s (first login must change password)",
			defaultAdmin.Username,
			cfg.DefaultAdminPass,
		)
	}

	objectStorageClient, err := storage.NewMinioClient(
		cfg.MinioEndpoint,
		cfg.MinioAccessKey,
		cfg.MinioSecretKey,
		cfg.MinioBucket,
		cfg.MinioUseSSL,
	)
	if err != nil {
		log.Fatalf("init object storage client failed: %v", err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 8*time.Second)
	if err := objectStorageClient.EnsureBucket(ctx); err != nil {
		cancel()
		log.Fatalf("init object storage bucket failed: %v", err)
	}
	cancel()

	ctx, cancel = context.WithTimeout(context.Background(), 8*time.Second)
	if err := objectStorageClient.EnsureBucketCORS(ctx, parseOrigins(cfg.FrontendOrigin)); err != nil {
		log.Printf("warn: init object storage bucket cors failed: %v", err)
	}
	cancel()

	tokenManager, err := auth.NewTokenManager(cfg.JWTSecret, cfg.AppBaseURL, cfg.AccessTokenTTL, cfg.RefreshTokenTTL)
	if err != nil {
		log.Fatalf("init token manager failed: %v", err)
	}

	apiServer := httpapi.NewServer(
		cfg,
		store.NewAttachmentRepo(db),
		store.NewDocumentRepo(db),
		store.NewDocumentVersionRepo(db),
		store.NewDocumentPermissionRepo(db),
		userRepo,
		store.NewRefreshTokenRepo(db),
		objectStorageClient,
		tokenManager,
	)
	router := apiServer.Router()

	httpServer := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           router,
		ReadTimeout:       15 * time.Second,
		ReadHeaderTimeout: 8 * time.Second,
		WriteTimeout:      20 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("alliance vault backend is listening on http://localhost:%s", cfg.AppPort)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, syscall.SIGINT, syscall.SIGTERM)
	<-stopChan

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		if closeErr := httpServer.Close(); closeErr != nil {
			log.Printf("force close failed: %v", closeErr)
		}
	}

	fmt.Println("alliance vault backend stopped")
}

func parseOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, part := range parts {
		origin := strings.TrimSpace(part)
		if origin == "" {
			continue
		}
		origins = append(origins, origin)
	}
	if len(origins) == 0 {
		return []string{
			"http://localhost:8080",
			"http://127.0.0.1:8080",
			"http://localhost:5173",
			"http://127.0.0.1:5173",
			"null",
		}
	}
	return origins
}
