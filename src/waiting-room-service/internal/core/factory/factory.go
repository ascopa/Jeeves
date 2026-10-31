package factory

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"jeeves/waiting-room-service/internal/adapters/inbound/rest"
	"jeeves/waiting-room-service/internal/adapters/outbound/admission"
	"jeeves/waiting-room-service/internal/adapters/outbound/redis"
	"jeeves/waiting-room-service/internal/adapters/outbound/repository"
	"jeeves/waiting-room-service/internal/core/service"
)

const (
	envHTTPPort           = "HTTP_PORT"
	envDBDSN              = "DB_DSN"
	envDBMaxConns         = "DB_MAX_CONNS"
	envDBStatementTimeout = "DB_STATEMENT_TIMEOUT"
	envRedisAddr          = "REDIS_ADDR"
	envAdmissionSecret    = "ADMISSION_SECRET"
	envAdmissionTTL       = "ADMISSION_TTL"
	envAdmitSeatsFactor   = "ADMIT_SEATS_FACTOR"

	defaultHTTPPort           = "8084"
	defaultDBMaxConns         = "5"
	defaultDBStatementTimeout = "2s"
	defaultAdmissionTTL       = "15m"
	defaultAdmitSeatsFactor   = "2"

	admitInterval = time.Second
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
	redisAddr, err := requiredEnv(envRedisAddr)
	if err != nil {
		return err
	}
	admissionSecret, err := requiredEnv(envAdmissionSecret)
	if err != nil {
		return err
	}
	admissionTTL, err := durationEnv(envAdmissionTTL, defaultAdmissionTTL)
	if err != nil {
		return err
	}
	seatsFactor, err := intEnv(envAdmitSeatsFactor, defaultAdmitSeatsFactor)
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

	redisClient, err := redis.NewClient(ctx, redisAddr)
	if err != nil {
		return err
	}
	defer redisClient.Close()

	srv := service.NewService(
		redis.NewQueueStore(redisClient),
		repository.NewEventRepository(pool),
		admission.NewTokenIssuer([]byte(admissionSecret)),
		admissionTTL,
		seatsFactor,
	)

	var admitter sync.WaitGroup
	admitter.Go(func() { srv.RunAdmitter(ctx, admitInterval) })
	defer admitter.Wait()

	router := rest.NewRouter(rest.NewController(srv))
	err = rest.Serve(ctx, router, envOr(envHTTPPort, defaultHTTPPort))
	stop()
	return err
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
