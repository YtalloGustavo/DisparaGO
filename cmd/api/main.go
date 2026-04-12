package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"disparago/internal/app"
	"disparago/internal/platform/migrations"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		postgresURL := os.Getenv("POSTGRES_URL")
		if postgresURL == "" {
			log.Fatal("POSTGRES_URL is required for migrations")
		}

		workdir, err := os.Getwd()
		if err != nil {
			log.Fatalf("resolve working directory: %v", err)
		}

		migrationsDir := filepath.Join(workdir, "migrations")
		if err := migrations.Run(ctx, postgresURL, migrationsDir); err != nil {
			log.Fatalf("run migrations: %v", err)
		}

		log.Println("migrations applied successfully")
		return
	}

	application, err := app.New(ctx)
	if err != nil {
		log.Fatalf("bootstrap app: %v", err)
	}

	go func() {
		if err := application.Start(); err != nil {
			log.Fatalf("start app: %v", err)
		}
	}()

	<-ctx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), application.Config.App.ShutdownTimeout)
	defer cancel()

	if err := application.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		os.Exit(1)
	}

	log.Println("application stopped")
	time.Sleep(100 * time.Millisecond)
}
