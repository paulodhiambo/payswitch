package integration

import (
	"context"
	"fmt"
	"log"
	"net/netip"
	"os"
	"testing"

	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/api/types/network"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/segmentio/kafka-go"
	tc "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type testEnv struct {
	pgPool    *pgxpool.Pool
	rdb       *redis.Client
	kafkaAddr string
	rp        tc.Container
	cleanup   func()
}

var env *testEnv

func (e *testEnv) createTopic(ctx context.Context, topic string) error {
	conn, err := kafka.Dial("tcp", e.kafkaAddr)
	if err != nil {
		return fmt.Errorf("dial broker: %w", err)
	}
	defer conn.Close()
	return conn.CreateTopics(kafka.TopicConfig{
		Topic:             topic,
		NumPartitions:     1,
		ReplicationFactor: 1,
	})
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	e, err := startContainers(ctx)
	if err != nil {
		log.Fatalf("failed to start containers: %v", err)
	}

	env = e
	code := m.Run()

	env.cleanup()
	os.Exit(code)
}

func startContainers(ctx context.Context) (*testEnv, error) {
	pg, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        "postgres:16-alpine",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "switch",
				"POSTGRES_PASSWORD": "switch",
				"POSTGRES_DB":       "switch_test",
			},
			WaitingFor: wait.ForAll(
				wait.ForLog("database system is ready to accept connections"),
				wait.ForListeningPort("5432/tcp"),
			),
		},
		Started: true,
	})
	if err != nil {
		return nil, fmt.Errorf("start postgres: %w", err)
	}

	pgPort, err := pg.MappedPort(ctx, "5432/tcp")
	if err != nil {
		return nil, fmt.Errorf("get postgres port: %w", err)
	}
	pgHost, err := pg.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("get postgres host: %w", err)
	}

	pgDSN := fmt.Sprintf("postgres://switch:switch@%s:%s/switch_test?sslmode=disable", pgHost, pgPort.Port())

	pool, err := pgxpool.New(ctx, pgDSN)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}

	for _, migration := range []string{
		"../../migrations/postgres/0001_init.sql",
		"../../migrations/postgres/0002_iso20022.sql",
		"../../migrations/postgres/0005_route_fields.sql",
	} {
		sql, err := os.ReadFile(migration)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", migration, err)
		}
		if _, err := pool.Exec(ctx, string(sql)); err != nil {
			return nil, fmt.Errorf("apply %s: %w", migration, err)
		}
	}

	kafkaHostPort := "19092"
	rp, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        "docker.redpanda.com/redpandadata/redpanda:v24.3.4",
			ExposedPorts: []string{"9092/tcp"},
			HostConfigModifier: func(hc *container.HostConfig) {
				hc.PortBindings = network.PortMap{
					network.MustParsePort("9092/tcp"): []network.PortBinding{
						{HostIP: netip.MustParseAddr("0.0.0.0"), HostPort: kafkaHostPort},
					},
				}
			},
			Cmd: []string{
				"redpanda", "start",
				"--smp", "1",
				"--overprovisioned",
				"--kafka-addr", "0.0.0.0:9092",
				"--advertise-kafka-addr", "localhost:" + kafkaHostPort,
				"--set", "redpanda.auto_create_topics_enabled=true",
			},
			WaitingFor: wait.ForAll(
				wait.ForLog("Successfully started Redpanda"),
				wait.ForListeningPort("9092/tcp"),
			),
		},
		Started: true,
	})
	if err != nil {
		return nil, fmt.Errorf("start redpanda: %w", err)
	}

	kafkaAddr := "localhost:" + kafkaHostPort

	redisC, err := tc.GenericContainer(ctx, tc.GenericContainerRequest{
		ContainerRequest: tc.ContainerRequest{
			Image:        "redis:7-alpine",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor: wait.ForAll(
				wait.ForLog("* Ready to accept connections"),
				wait.ForListeningPort("6379/tcp"),
			),
		},
		Started: true,
	})
	if err != nil {
		return nil, fmt.Errorf("start redis: %w", err)
	}

	redisPort, err := redisC.MappedPort(ctx, "6379/tcp")
	if err != nil {
		return nil, fmt.Errorf("get redis port: %w", err)
	}
	redisHost, err := redisC.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("get redis host: %w", err)
	}
	redisAddr := fmt.Sprintf("%s:%s", redisHost, redisPort.Port())

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	log.Printf("integration test environment ready:")
	log.Printf("  postgres: %s", pgDSN)
	log.Printf("  kafka:    %s", kafkaAddr)
	log.Printf("  redis:    %s", redisAddr)

	cleanup := func() {
		pool.Close()
		rdb.Close()
		redisC.Terminate(ctx)
		rp.Terminate(ctx)
		pg.Terminate(ctx)
	}

	return &testEnv{
		pgPool:    pool,
		rdb:       rdb,
		kafkaAddr: kafkaAddr,
		rp:        rp,
		cleanup:   cleanup,
	}, nil
}


