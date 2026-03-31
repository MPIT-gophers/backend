package config

import (
	"fmt"

	"github.com/caarlos0/env/v11"
	_ "github.com/joho/godotenv/autoload"
)

type Config struct {
	App      AppConfig      `envPrefix:"APP_"`
	HTTP     HTTPConfig     `envPrefix:"HTTP_"`
	Postgres PostgresConfig `envPrefix:"POSTGRES_"`
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

func Load() (Config, error) {
	cfg := Config{}
	if err := env.Parse(&cfg); err != nil {
		return Config{}, err
	}

	return cfg, nil
}
