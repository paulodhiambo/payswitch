package eventbus

import (
	"context"

	"github.com/segmentio/kafka-go"
)

type Handler func(ctx context.Context, msg kafka.Message) error

func Consume(ctx context.Context, brokers []string, topic, group string, h Handler) error {
	r := kafka.NewReader(kafka.ReaderConfig{
		Brokers: brokers,
		Topic:   topic,
		GroupID: group,
	})
	defer r.Close()

	for {
		msg, err := r.FetchMessage(ctx)
		if err != nil {
			return err
		}
		if err := h(ctx, msg); err != nil {
			continue
		}
		if err := r.CommitMessages(ctx, msg); err != nil {
			return err
		}
	}
}
