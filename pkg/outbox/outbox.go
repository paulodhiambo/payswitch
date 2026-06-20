package outbox

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

func Write(ctx context.Context, tx pgx.Tx, topic, key string, event any) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO outbox (topic, msg_key, payload) VALUES ($1, $2, $3)`,
		topic, key, payload)
	return err
}
