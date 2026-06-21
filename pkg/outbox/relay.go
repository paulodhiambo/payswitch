package outbox

import (
	"context"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"switch/pkg/eventbus"
)

type Relay struct {
	db        *pgxpool.Pool
	publisher *eventbus.Producer
}

func NewRelay(db *pgxpool.Pool, publisher *eventbus.Producer) *Relay {
	return &Relay{db: db, publisher: publisher}
}

func (r *Relay) Run(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := r.flushBatch(ctx); err != nil {
				log.Printf("outbox relay error: %v", err)
			}
		}
	}
}

func (r *Relay) flushBatch(ctx context.Context) error {
	tx, err := r.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT id, topic, msg_key, payload FROM outbox
		WHERE status = 'PENDING' ORDER BY id LIMIT 500
		FOR UPDATE SKIP LOCKED`)
	if err != nil {
		return err
	}

	var ids []int64
	for rows.Next() {
		var id int64
		var topic, key string
		var payload []byte
		if err := rows.Scan(&id, &topic, &key, &payload); err != nil {
			return err
		}
		if err := r.publisher.Publish(ctx, topic, key, payload); err != nil {
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()

	if len(ids) > 0 {
		if _, err := tx.Exec(ctx,
			`UPDATE outbox SET status = 'PUBLISHED', published_at = now() WHERE id = ANY($1)`,
			ids); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
