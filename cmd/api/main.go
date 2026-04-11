package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"disparago/internal/app"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
