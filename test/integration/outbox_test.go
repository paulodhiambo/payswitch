package integration

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"switch/internal/orchestrator/domain"
	"switch/pkg/eventbus"
	"switch/pkg/outbox"
)

var topicSeqMu sync.Mutex
var topicSeq int

func TestKafka_DirectProduceConsume(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	topic := "direct-" + uuid.New().String()

	err := env.createTopic(ctx, topic)
	require.NoError(t, err, "create topic via kafka-go API")

	key := "test-key"
	payload := []byte(`{"hello":"world"}`)

	producer := eventbus.NewProducer([]string{env.kafkaAddr})
	defer producer.Close()

	err = producer.Publish(ctx, topic, key, payload)
	require.NoError(t, err, "should publish message directly")

	consumer := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{env.kafkaAddr},
		Topic:     topic,
		Partition: 0,
		MinBytes:  1,
		MaxBytes:  10e6,
		MaxWait:   5 * time.Second,
	})
	defer consumer.Close()

	msg, err := consumer.ReadMessage(ctx)
	require.NoError(t, err, "should consume message")
	assert.Equal(t, key, string(msg.Key))
	assert.Equal(t, payload, msg.Value)
}

func TestOutboxRelay_PublishesEvent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	topic := "outbox-" + uuid.New().String()

	producer := eventbus.NewProducer([]string{env.kafkaAddr})
	defer producer.Close()

	consumer := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{env.kafkaAddr},
		Topic:     topic,
		GroupID:   "test-group-" + uuid.New().String(),
		Partition: 0,
		MinBytes:  1,
		MaxBytes:  10e6,
		MaxWait:   3 * time.Second,
	})
	defer consumer.Close()

	relay := outbox.NewRelay(env.pgPool, producer)
	go relay.Run(ctx, 500*time.Millisecond)

	time.Sleep(2 * time.Second)

	key := "event-key-" + uuid.New().String()
	event := domain.PaymentEvent{
		PaymentID:  uuid.New().String(),
		EndToEndID: "e2e-outbox-test",
		ToStatus:   domain.StatusReceived,
		SourceBIC:  "BANKUS33",
		DestBIC:    "BANKDEFF",
		Amount:     123_45,
		Currency:   "USD",
	}

	err := outbox.Write(ctx, env.pgPool, topic, key, event)
	require.NoError(t, err)

	var msg kafka.Message
	select {
	case <-ctx.Done():
		t.Fatal("timeout waiting for outbox event")
	default:
		msg, err = consumer.ReadMessage(ctx)
		require.NoError(t, err)
	}

	assert.Equal(t, key, string(msg.Key))
	assert.Contains(t, string(msg.Value), event.EndToEndID)
	assert.Contains(t, string(msg.Value), event.PaymentID)
}

func TestOutboxRelay_MultipleEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	topic := "multi-" + uuid.New().String()

	producer := eventbus.NewProducer([]string{env.kafkaAddr})
	defer producer.Close()

	consumer := kafka.NewReader(kafka.ReaderConfig{
		Brokers:   []string{env.kafkaAddr},
		Topic:     topic,
		GroupID:   "test-group-" + uuid.New().String(),
		Partition: 0,
		MinBytes:  1,
		MaxBytes:  10e6,
		MaxWait:   2 * time.Second,
	})
	defer consumer.Close()

	relay := outbox.NewRelay(env.pgPool, producer)
	go relay.Run(ctx, 500*time.Millisecond)

	time.Sleep(2 * time.Second)

	n := 5
	for i := 0; i < n; i++ {
		key := "key-" + uuid.New().String()
		err := outbox.Write(ctx, env.pgPool, topic, key, domain.PaymentEvent{
			PaymentID:  uuid.New().String(),
			EndToEndID: "multi-" + key,
			ToStatus:   domain.StatusReceived,
		})
		require.NoError(t, err)
	}

	received := make(map[string]bool)
	for i := 0; i < n; i++ {
		msg, err := consumer.ReadMessage(ctx)
		require.NoError(t, err)
		received[string(msg.Key)] = true
	}

	assert.Len(t, received, n, "expected %d unique events", n)
}
