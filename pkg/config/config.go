package config

import "github.com/spf13/viper"

type Config struct {
	HTTPAddr      string   `mapstructure:"HTTP_ADDR"`
	GRPCAddr      string   `mapstructure:"GRPC_ADDR"`
	PostgresDSN   string   `mapstructure:"POSTGRES_DSN"`
	RedisAddr     string   `mapstructure:"REDIS_ADDR"`
	KafkaBrokers  []string `mapstructure:"KAFKA_BROKERS"`
	OTLPEndpoint  string   `mapstructure:"OTLP_ENDPOINT"`
	MetricsAddr   string   `mapstructure:"METRICS_ADDR"`
	ScyllaHosts   []string `mapstructure:"SCYLLA_HOSTS"`
	ScyllaKeyspace string  `mapstructure:"SCYLLA_KEYSPACE"`
	TLSCertFile   string   `mapstructure:"TLS_CERT_FILE"`
	TLSKeyFile    string   `mapstructure:"TLS_KEY_FILE"`
	TLSCAFile     string   `mapstructure:"TLS_CA_FILE"`
	ComplianceAddr string  `mapstructure:"COMPLIANCE_ADDR"`
	LookupAddr    string   `mapstructure:"LOOKUP_ADDR"`
	SettlementAddr   string `mapstructure:"SETTLEMENT_ADDR"`
	QuotingAddr     string `mapstructure:"QUOTING_ADDR"`
	NotificationAddr string `mapstructure:"NOTIFICATION_ADDR"`
}

func Load() (*Config, error) {
	v := viper.New()
	v.AutomaticEnv()
	v.SetDefault("HTTP_ADDR", ":8080")
	v.SetDefault("GRPC_ADDR", ":9090")
	v.SetDefault("POSTGRES_DSN", "postgres://switch:switch@localhost:5432/switch?sslmode=disable")
	v.SetDefault("REDIS_ADDR", "")
	v.SetDefault("METRICS_ADDR", ":9095")
	v.SetDefault("SCYLLA_KEYSPACE", "switch")
	v.SetDefault("COMPLIANCE_ADDR", "localhost:9091")
	v.SetDefault("LOOKUP_ADDR", "localhost:9092")
	v.SetDefault("SETTLEMENT_ADDR", "localhost:9093")
	v.SetDefault("QUOTING_ADDR", "localhost:9094")
	v.SetDefault("NOTIFICATION_ADDR", "")

	var c Config
	if err := v.Unmarshal(&c); err != nil {
		return nil, err
	}
	return &c, nil
}
