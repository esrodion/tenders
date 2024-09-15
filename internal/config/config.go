package config

import (
	"fmt"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	ServerAddress string `env:"SERVER_ADDRESS" envDefault:"0.0.0.0:8080"`
	LogLevel      string `env:"LOG_LEVEL" envDefault:"DEBUG"`
	PostgresConfig
}

func NewConfig() (*Config, error) {
	config := &Config{}

	err := env.Parse(config)
	if err != nil {
		err = fmt.Errorf("config.NewConfig: %w", err)
	}
	return config, err
}

type PostgresConfig struct {
	Conn            string `env:"POSTGRES_CONN" envDefault:"postgres://test:test@db:5432/test?sslmode=disable"`
	Host            string `env:"POSTGRES_HOST" envDefault:"db"`
	Port            string `env:"POSTGRES_PORT" envDefault:"8812"`
	Username        string `env:"POSTGRES_USERNAME" envDefault:"admin"`
	Password        string `env:"POSTGRES_PASSWORD" envDefault:"quest"`
	Database        string `env:"POSTGRES_DATABASE" envDefault:"qdb"`
	AutoMigrateUp   string `env:"AUTO_MIGRATE_UP" envDefault:"true"`
	AutoMigrateDown string `env:"AUTO_MIGRATE_DOWN" envDefault:"false"`
	MigrationsURL   string `env:"MIGRATIONS_URL" envDefault:"file://D:/Work/Avito/Repos/Internship/zadanie-6105/internal/repository/db/migrations"`
}

//"file://D:/Work/Avito/Repos/Internship/zadanie-6105/internal/repository/db/migrations"

func NewPostgresConfig() (*PostgresConfig, error) {
	config := &PostgresConfig{}

	err := env.Parse(config)
	if err != nil {
		err = fmt.Errorf("config.NewPostgresConfig: %w", err)
	}
	return config, err
}
