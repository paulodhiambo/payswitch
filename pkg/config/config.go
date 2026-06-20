package config

import (
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
)

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
	ComplianceAddr      string `mapstructure:"COMPLIANCE_ADDR"`
	LookupAddr          string `mapstructure:"LOOKUP_ADDR"`
	SettlementAddr      string `mapstructure:"SETTLEMENT_ADDR"`
	QuotingAddr         string `mapstructure:"QUOTING_ADDR"`
	NotificationAddr    string `mapstructure:"NOTIFICATION_ADDR"`
	RoutingAddr         string `mapstructure:"ROUTING_ADDR"`
	ReconciliationAddr  string `mapstructure:"RECONCILIATION_ADDR"`
	// Portal API
	CSRFSecret   string `mapstructure:"CSRF_SECRET"`   // hex-encoded 32-byte key; random if unset (tokens lost on restart)
	PortalOrigin string `mapstructure:"PORTAL_ORIGIN"` // allowed CORS origin, e.g. https://portal.example.com
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
	v.SetDefault("KAFKA_BROKERS", []string{})
	v.SetDefault("SCYLLA_HOSTS", []string{})

	_ = v.BindEnv("KAFKA_BROKERS")
	_ = v.BindEnv("SCYLLA_HOSTS")

	var c Config
	if err := v.Unmarshal(&c, viper.DecodeHook(
		mapstructure.ComposeDecodeHookFunc(
			mapstructure.StringToSliceHookFunc(","),
		),
	)); err != nil {
		return nil, err
	}
	return &c, nil
}
