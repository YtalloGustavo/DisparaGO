package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	App         AppConfig
	Auth        AuthConfig
	InternalAPI InternalAPIConfig
	Humanizer   HumanizerConfig
	Webhook     WebhookConfig
	Log         LogConfig
	Retry       RetryConfig
	Scheduler   SchedulerConfig
	Postgres    PostgresConfig
	Redis       RedisConfig
	EvolutionGO EvolutionGOConfig
}

type AppConfig struct {
	Name            string
	Env             string
	Host            string
	Port            string
	PublicURL       string
	ReadTimeout     time.Duration
	WriteTimeout    time.Duration
	ShutdownTimeout time.Duration
}

type LogConfig struct {
	Level string
}

type AuthConfig struct {
	BootstrapCompanyName  string
	OperatorUsername      string
	OperatorPassword      string
	OperatorDisplayName   string
	SuperadminUsername    string
	SuperadminPassword    string
	SuperadminDisplayName string
	Secret                string
	TokenTTL              time.Duration
}

type InternalAPIConfig struct {
	Key string
}

type HumanizerConfig struct {
	Enabled          bool
	BaseDelayMin     time.Duration
	BaseDelayMax     time.Duration
	ProviderDelayMin time.Duration
	ProviderDelayMax time.Duration
	BurstSizeMin     int
	BurstSizeMax     int
	BurstPauseMin    time.Duration
	BurstPauseMax    time.Duration
	InitialDelayMin  time.Duration
	InitialDelayMax  time.Duration
}

type WebhookConfig struct {
	TokenSecret          string
	DefaultSubscriptions []string
}

type RetryConfig struct {
	MaxAttempts int
	Delay       time.Duration
}

type SchedulerConfig struct {
	PollInterval time.Duration
	BatchSize    int
}

type PostgresConfig struct {
	URL string
}

type RedisConfig struct {
	URL                   string
	CampaignMessagesQueue string
}

type EvolutionGOConfig struct {
	BaseURL string
	APIKey  string
	Timeout time.Duration
}

func Load() (Config, error) {
	_ = godotenv.Load()

	cfg := Config{
		App: AppConfig{
			Name:            getEnv("APP_NAME", "DisparaGO"),
			Env:             getEnv("APP_ENV", "development"),
			Host:            getEnv("APP_HOST", "0.0.0.0"),
			Port:            getEnv("APP_PORT", "8080"),
			PublicURL:       strings.TrimRight(getEnv("APP_PUBLIC_URL", ""), "/"),
			ReadTimeout:     getEnvDurationSeconds("APP_READ_TIMEOUT", 15),
			WriteTimeout:    getEnvDurationSeconds("APP_WRITE_TIMEOUT", 15),
			ShutdownTimeout: getEnvDurationSeconds("APP_SHUTDOWN_TIMEOUT", 10),
		},
		Auth: AuthConfig{
			BootstrapCompanyName:  getEnv("AUTH_BOOTSTRAP_COMPANY_NAME", "Default Company"),
			OperatorUsername:      getEnv("AUTH_USERNAME", "admin"),
			OperatorPassword:      getEnv("AUTH_PASSWORD", "change-me"),
			OperatorDisplayName:   getEnv("AUTH_DISPLAY_NAME", "Operator"),
			SuperadminUsername:    getEnv("SUPERADMIN_USERNAME", "superadmin"),
			SuperadminPassword:    getEnv("SUPERADMIN_PASSWORD", "change-me-superadmin"),
			SuperadminDisplayName: getEnv("SUPERADMIN_DISPLAY_NAME", "Superadmin"),
			Secret:                getEnv("AUTH_SECRET", "disparago-dev-secret"),
			TokenTTL:              getEnvDurationHours("AUTH_TOKEN_TTL_HOURS", 12),
		},
		InternalAPI: InternalAPIConfig{
			Key: getEnv("INTERNAL_API_KEY", ""),
		},
		Humanizer: HumanizerConfig{
			Enabled:          getEnvBool("HUMANIZER_ENABLED", true),
			BaseDelayMin:     getEnvDurationSeconds("HUMANIZER_BASE_DELAY_MIN_SECONDS", 9),
			BaseDelayMax:     getEnvDurationSeconds("HUMANIZER_BASE_DELAY_MAX_SECONDS", 18),
			ProviderDelayMin: getEnvDurationMilliseconds("HUMANIZER_PROVIDER_DELAY_MIN_MS", 1500),
			ProviderDelayMax: getEnvDurationMilliseconds("HUMANIZER_PROVIDER_DELAY_MAX_MS", 5000),
			BurstSizeMin:     getEnvInt("HUMANIZER_BURST_SIZE_MIN", 4),
			BurstSizeMax:     getEnvInt("HUMANIZER_BURST_SIZE_MAX", 8),
			BurstPauseMin:    getEnvDurationSeconds("HUMANIZER_BURST_PAUSE_MIN_SECONDS", 45),
			BurstPauseMax:    getEnvDurationSeconds("HUMANIZER_BURST_PAUSE_MAX_SECONDS", 120),
			InitialDelayMin:  getEnvDurationSeconds("HUMANIZER_INITIAL_DELAY_MIN_SECONDS", 3),
			InitialDelayMax:  getEnvDurationSeconds("HUMANIZER_INITIAL_DELAY_MAX_SECONDS", 8),
		},
		Webhook: WebhookConfig{
			TokenSecret:          getEnv("WEBHOOK_TOKEN_SECRET", "disparago-webhook-secret"),
			DefaultSubscriptions: getEnvCSV("WEBHOOK_DEFAULT_SUBSCRIPTIONS", []string{"ALL"}),
		},
		Log: LogConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
		Retry: RetryConfig{
			MaxAttempts: getEnvInt("RETRY_MAX_ATTEMPTS", 3),
			Delay:       getEnvDurationSeconds("RETRY_DELAY_SECONDS", 15),
		},
		Scheduler: SchedulerConfig{
			PollInterval: getEnvDurationSeconds("SCHEDULER_POLL_INTERVAL_SECONDS", 30),
			BatchSize:    getEnvInt("SCHEDULER_BATCH_SIZE", 25),
		},
		Postgres: PostgresConfig{
			URL: getEnv("POSTGRES_URL", ""),
		},
		Redis: RedisConfig{
			URL:                   getEnv("REDIS_URL", ""),
			CampaignMessagesQueue: getEnv("REDIS_QUEUE_CAMPAIGN_MESSAGES", "queue:campaign-messages"),
		},
		EvolutionGO: EvolutionGOConfig{
			BaseURL: getEnv("EVOLUTIONGO_BASE_URL", ""),
			APIKey:  getEnv("EVOLUTIONGO_API_KEY", ""),
			Timeout: getEnvDurationSeconds("EVOLUTIONGO_TIMEOUT", 30),
		},
	}

	if cfg.Postgres.URL == "" {
		return Config{}, fmt.Errorf("POSTGRES_URL is required")
	}

	if cfg.Redis.URL == "" {
		return Config{}, fmt.Errorf("REDIS_URL is required")
	}

	if cfg.EvolutionGO.BaseURL == "" {
		return Config{}, fmt.Errorf("EVOLUTIONGO_BASE_URL is required")
	}

	cfg.Humanizer.normalize()

	return cfg, nil
}

func (c *HumanizerConfig) normalize() {
	if c.BaseDelayMax < c.BaseDelayMin {
		c.BaseDelayMax = c.BaseDelayMin
	}

	if c.ProviderDelayMax < c.ProviderDelayMin {
		c.ProviderDelayMax = c.ProviderDelayMin
	}

	if c.BurstSizeMax < c.BurstSizeMin {
		c.BurstSizeMax = c.BurstSizeMin
	}

	if c.BurstPauseMax < c.BurstPauseMin {
		c.BurstPauseMax = c.BurstPauseMin
	}

	if c.InitialDelayMax < c.InitialDelayMin {
		c.InitialDelayMax = c.InitialDelayMin
	}
}

func getEnv(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	return value
}

func getEnvDurationSeconds(key string, fallback int) time.Duration {
	seconds := getEnvInt(key, fallback)
	return time.Duration(seconds) * time.Second
}

func getEnvDurationHours(key string, fallback int) time.Duration {
	hours := getEnvInt(key, fallback)
	return time.Duration(hours) * time.Hour
}

func getEnvDurationMilliseconds(key string, fallback int) time.Duration {
	milliseconds := getEnvInt(key, fallback)
	return time.Duration(milliseconds) * time.Millisecond
}

func getEnvInt(key string, fallback int) int {
	raw := getEnv(key, strconv.Itoa(fallback))
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return value
}

func getEnvBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(getEnv(key, "")))
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	case "":
		return fallback
	default:
		return fallback
	}
}

func getEnvCSV(key string, fallback []string) []string {
	raw := strings.TrimSpace(getEnv(key, ""))
	if raw == "" {
		return fallback
	}

	parts := strings.Split(raw, ",")
	items := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			items = append(items, value)
		}
	}

	if len(items) == 0 {
		return fallback
	}

	return items
}
