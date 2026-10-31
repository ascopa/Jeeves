package factory

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"jeeves/card-provider-mock/internal/adapters/inbound/rest"
	"jeeves/card-provider-mock/internal/adapters/outbound/memory"
	"jeeves/card-provider-mock/internal/core/service"
)

const (
	envHTTPPort = "HTTP_PORT"
	envAPIKey   = "CARD_PROVIDER_API_KEY"

	defaultHTTPPort = "9090"
)

type Factory struct{}

func New() *Factory {
	return &Factory{}
}

func (f *Factory) Run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	apiKey := os.Getenv(envAPIKey)
	if apiKey == "" {
		return fmt.Errorf("environment variable %s is not set", envAPIKey)
	}

	srv := service.NewService(memory.NewCardStore(), memory.NewChargeStore())

	router := rest.NewRouter(rest.NewController(srv), apiKey)
	return rest.Serve(ctx, router, envOr(envHTTPPort, defaultHTTPPort))
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
