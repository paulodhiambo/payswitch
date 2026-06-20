package outbox

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5/pgconn"
)

type Execer interface {
	Exec(ctx context.Context, sql string, args ...interface{}) (pgconn.CommandTag, error)
}

func Write(ctx context.Context, e Execer, topic, key string, event any) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = e.Exec(ctx,
		`INSERT INTO outbox (topic, msg_key, payload) VALUES ($1, $2, $3)`,
		topic, key, payload)
	return err
}
