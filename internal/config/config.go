package config

import (
	"fmt"
	"log/slog"
	"net"
	"reflect"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/go-playground/validator/v10/non-standard/validators"
	"github.com/ilyakaznacheev/cleanenv"
)

type Config struct {
	HTTPAddr                string        `env:"HTTP_ADDR" env-required:"true" validate:"notblank,listen_addr"`
	HTTPReadTimeout         time.Duration `env:"HTTP_READ_TIMEOUT" env-required:"true" validate:"gt=0"`
	HTTPReadHeaderTimeout   time.Duration `env:"HTTP_READ_HEADER_TIMEOUT" env-required:"true" validate:"gt=0"`
	HTTPWriteTimeout        time.Duration `env:"HTTP_WRITE_TIMEOUT" env-required:"true" validate:"gt=0"`
	HTTPIdleTimeout         time.Duration `env:"HTTP_IDLE_TIMEOUT" env-required:"true" validate:"gt=0"`
	LogLevel                slog.Level    `env:"LOG_LEVEL" env-required:"true"`
	ShutdownTimeout         time.Duration `env:"SHUTDOWN_TIMEOUT" env-required:"true" validate:"gt=0"`
	DatabaseURL             string        `env:"DATABASE_URL" env-required:"true" validate:"notblank"`
	DatabaseMaxConns        int32         `env:"DATABASE_MAX_CONNS" env-required:"true" validate:"gte=1"`
	DatabaseMinConns        int32         `env:"DATABASE_MIN_CONNS" env-required:"true" validate:"gte=0,ltefield=DatabaseMaxConns"`
	DatabaseMaxConnLifetime time.Duration `env:"DATABASE_MAX_CONN_LIFETIME" env-required:"true" validate:"gt=0"`
	DatabaseConnectTimeout  time.Duration `env:"DATABASE_CONNECT_TIMEOUT" env-required:"true" validate:"gt=0"`
	DatabaseQueryTimeout    time.Duration `env:"DATABASE_QUERY_TIMEOUT" env-required:"true" validate:"gt=0"`
	IdempotencyTTL          time.Duration `env:"IDEMPOTENCY_TTL" env-required:"true" validate:"gt=0"`
	OTelServiceName         string        `env:"OTEL_SERVICE_NAME" env-required:"true" validate:"notblank"`
	OTelEndpoint            string        `env:"OTEL_EXPORTER_OTLP_ENDPOINT" env-required:"true" validate:"http_url"`
	OTelProtocol            string        `env:"OTEL_EXPORTER_OTLP_PROTOCOL" env-required:"true" validate:"oneof=grpc http/protobuf"`
	OTelResourceAttributes  string        `env:"OTEL_RESOURCE_ATTRIBUTES" env-required:"true" validate:"notblank"`
	OTelSampler             string        `env:"OTEL_TRACES_SAMPLER" env-required:"true" validate:"oneof=always_on always_off parentbased_always_on parentbased_always_off traceidratio parentbased_traceidratio"`
	OTelSamplerArg          float64       `env:"OTEL_TRACES_SAMPLER_ARG" env-default:"1" validate:"gte=0,lte=1"`
	ServiceVersion          string        `env:"SERVICE_VERSION" env-default:"dev" validate:"notblank"`
}

func Load() (Config, error) {
	var cfg Config
	if err := cleanenv.ReadEnv(&cfg); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}

	validate := validator.New()
	validate.RegisterTagNameFunc(func(field reflect.StructField) string {
		return field.Tag.Get("env")
	})
	if err := validate.RegisterValidation("notblank", validators.NotBlank); err != nil {
		return Config{}, fmt.Errorf("register config validation: %w", err)
	}
	if err := validate.RegisterValidation("listen_addr", validListenAddr); err != nil {
		return Config{}, fmt.Errorf("register config validation: %w", err)
	}
	if err := validate.Struct(cfg); err != nil {
		return Config{}, fmt.Errorf("validate config: %w", err)
	}
	return cfg, nil
}

func validListenAddr(field validator.FieldLevel) bool {
	_, port, err := net.SplitHostPort(field.Field().String())
	if err != nil {
		return false
	}
	portNumber, err := strconv.Atoi(port)
	return err == nil && portNumber >= 1 && portNumber <= 65535
}
