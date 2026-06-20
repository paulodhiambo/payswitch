package config

import "github.com/spf13/viper"

type Config struct {
	HTTPAddr     string   `mapstructure:"HTTP_ADDR"`
	PostgresDSN  string   `mapstructure:"POSTGRES_DSN"`
	RedisAddr    string   `mapstructure:"REDIS_ADDR"`
	KafkaBrokers []string `mapstructure:"KAFKA_BROKERS"`
	OTLPEndpoint string   `mapstructure:"OTLP_ENDPOINT"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.AutomaticEnv()
	v.SetDefault("HTTP_ADDR", ":8080")
	v.SetDefault("POSTGRES_DSN", "postgres://switch:switch@localhost:5432/switch?sslmode=disable")

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}
	return &c, nil
}
