package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"example.com/guardkit/react-go-api/internal/auth"
	"example.com/guardkit/react-go-api/internal/notes"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the local /api/health and exit (for the container healthcheck)")
	flag.Parse()
	if *healthcheck {
		if err := probe(listenAddr()); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		slog.Error("api stopped", "kind", "startup or server failure", "reason", err.Error())
		os.Exit(1)
	}
}

// listenAddr defaults to loopback; the container sets API_LISTEN_ADDR=:3000.
func listenAddr() string {
	if addr := os.Getenv("API_LISTEN_ADDR"); addr != "" {
		return addr
	}
	return "127.0.0.1:3000"
}

func probe(addr string) error {
	host := addr
	if len(host) > 0 && host[0] == ':' {
		host = "127.0.0.1" + host
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + host + "/api/health")
	if err != nil {
		return fmt.Errorf("health probe: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health probe: status %d", resp.StatusCode)
	}
	return nil
}

func required(name string) (string, error) {
	value := os.Getenv(name)
	if value == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return value, nil
}

func run() error {
	url, err := required("DATABASE_URL")
	if err != nil {
		return err
	}
	issuerList, err := required("OIDC_ISSUERS")
	if err != nil {
		return err
	}
	issuers, err := auth.ParseIssuers(issuerList)
	if err != nil {
		return err
	}
	audience, err := required("OIDC_AUDIENCE")
	if err != nil {
		return err
	}
	jwksURL, err := required("OIDC_JWKS_URL")
	if err != nil {
		return err
	}
	runID := os.Getenv("NOTES_RUN_ID")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	verifier, err := auth.New(auth.Config{Issuers: issuers, Audience: audience, JWKSURL: jwksURL}, nil)
	if err != nil {
		return fmt.Errorf("configure token checks: %w", err)
	}
	startupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	pool, err := pgxpool.New(startupCtx, url)
	if err != nil {
		cancel()
		return fmt.Errorf("configure database: %w", err)
	}
	defer pool.Close()
	if err := pool.Ping(startupCtx); err != nil {
		cancel()
		return fmt.Errorf("connect database: %w", err)
	}
	cancel()

	server := &http.Server{
		Addr:              listenAddr(),
		Handler:           notes.Routes(notes.PGStore{Pool: pool}, verifier, runID),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	result := make(chan error, 1)
	go func() { result <- server.ListenAndServe() }()
	slog.Info("api listening", "addr", server.Addr, "issuers", len(issuers))
	select {
	case err := <-result:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve api: %w", err)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shut down api: %w", err)
		}
		if err := <-result; !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve api: %w", err)
		}
		return nil
	}
}
