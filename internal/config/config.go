package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	App      AppConfig      `envPrefix:"APP_"`
	HTTP     HTTPConfig     `envPrefix:"HTTP_"`
	Postgres PostgresConfig `envPrefix:"POSTGRES_"`
	JWT      JWTConfig      `envPrefix:"JWT_"`
	MAX      MAXConfig      `envPrefix:"MAX_"`
	N8N      N8NConfig      `envPrefix:"N8N_"`
	Casbin   CasbinConfig   `envPrefix:"CASBIN_"`
	Log      LogConfig      `envPrefix:"LOG_"`
}

type AppConfig struct {
	Name string `env:"NAME" envDefault:"mpit2026-reg"`
	Env  string `env:"ENV" envDefault:"local"`
}

type HTTPConfig struct {
	Host string `env:"HOST" envDefault:"0.0.0.0"`
	Port string `env:"PORT" envDefault:"8080"`
}

func (c HTTPConfig) Addr() string {
	return fmt.Sprintf("%s:%s", c.Host, c.Port)
}

type PostgresConfig struct {
	Host           string `env:"HOST" envDefault:"localhost"`
	Port           int    `env:"PORT" envDefault:"5432"`
	User           string `env:"USER" envDefault:"postgres"`
	Password       string `env:"PASSWORD" envDefault:"postgres"`
	DBName         string `env:"DB" envDefault:"mpit2026_reg"`
	SSLMode        string `env:"SSLMODE" envDefault:"disable"`
	MaxConns       int32  `env:"MAX_CONNS" envDefault:"10"`
	AutoMigrate    bool   `env:"AUTO_MIGRATE" envDefault:"true"`
	MigrationsPath string `env:"MIGRATIONS_PATH" envDefault:"file://migrations"`
}

func (c PostgresConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		c.User,
		c.Password,
		c.Host,
		c.Port,
		c.DBName,
		c.SSLMode,
	)
}

type LogConfig struct {
	Level string `env:"LEVEL" envDefault:"info"`
}

type JWTConfig struct {
	Secret string        `env:"SECRET" envDefault:"local-dev-secret"`
	TTL    time.Duration `env:"TTL" envDefault:"168h"`
}

type MAXConfig struct {
	BotToken    string `env:"BOT_TOKEN"`
	BotUsername string `env:"BOT_USERNAME"`
	APIBaseURL  string `env:"API_BASE_URL" envDefault:"https://platform-api.max.ru"`
}

type N8NConfig struct {
	PointSearchWebhookURL string        `env:"POINT_SEARCH_WEBHOOK_URL" envDefault:"https://mpit-bot.kostya1024.ru/webhook/point-search"`
	Timeout               time.Duration `env:"TIMEOUT" envDefault:"15s"`
}

type CasbinConfig struct {
	ModelPath  string `env:"MODEL_PATH" envDefault:"configs/model/casbin_model.conf"`
	PolicyPath string `env:"POLICY_PATH" envDefault:"configs/model/casbin_policy.csv"`
}

func Load() (Config, error) {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
