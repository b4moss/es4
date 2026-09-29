package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/b4moss/es4/packages/go/internal/server"
	"github.com/b4moss/es4/packages/go/pkg/es4"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("es4-server: %v", err)
	}
}

func run() error {
	opts, err := server.LoadOptions(nil)
	if err != nil {
		return err
	}
	addr, err := server.ListenAddr(nil)
	if err != nil {
		return err
	}

	ctx := context.Background()
	db, err := es4.Open(ctx, opts)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	srv := server.New(db)
	defer srv.Close()

	httpSrv := &http.Server{
		Addr:              addr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Printf("es4-server listening on %s", addr)
		errCh <- httpSrv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		log.Printf("es4-server shutting down (%s)", sig)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == nil || err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}
