package factory

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"jeeves/search-event-service/internal/adapters/inbound/rest"
	"jeeves/search-event-service/internal/adapters/outbound/repository"
	"jeeves/search-event-service/internal/core/service"
)

const (
	envHTTPPort           = "HTTP_PORT"
	envDBDSN              = "DB_DSN"
	envDBMaxConns         = "DB_MAX_CONNS"
	envDBStatementTimeout = "DB_STATEMENT_TIMEOUT"

	defaultHTTPPort           = "8081"
	defaultDBMaxConns         = "20"
	defaultDBStatementTimeout = "2s"
)

type Factory struct{}

func New() *Factory {
	return &Factory{}
}

func (f *Factory) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	dsn, err := requiredEnv(envDBDSN)
	if err != nil {
		return err
	}
	poolCfg, err := poolConfig()
	if err != nil {
		return err
	}

	pool, err := repository.NewPool(ctx, dsn, poolCfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	srv := service.NewService(repository.NewEventRepository(pool))

	router := rest.NewRouter(rest.NewController(srv))
	return rest.Serve(ctx, router, envOr(envHTTPPort, defaultHTTPPort))
}

func poolConfig() (repository.PoolConfig, error) {
	maxConns, err := intEnv(envDBMaxConns, defaultDBMaxConns)
	if err != nil {
		return repository.PoolConfig{}, err
	}
	timeout, err := durationEnv(envDBStatementTimeout, defaultDBStatementTimeout)
	if err != nil {
		return repository.PoolConfig{}, err
	}
	return repository.PoolConfig{MaxConns: int32(maxConns), StatementTimeout: timeout}, nil
}

func requiredEnv(key string) (string, error) {
	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("environment variable %s is not set", key)
	}
	return value, nil
}

func durationEnv(key, fallback string) (time.Duration, error) {
	value, err := time.ParseDuration(envOr(key, fallback))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("environment variable %s must be a positive duration", key)
	}
	return value, nil
}

func intEnv(key, fallback string) (int, error) {
	value, err := strconv.Atoi(envOr(key, fallback))
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("environment variable %s must be a positive integer", key)
	}
	return value, nil
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
