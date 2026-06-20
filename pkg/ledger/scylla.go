package ledger

import (
	"fmt"
	"time"

	"github.com/gocql/gocql"
)

type Entry struct {
	ParticipantID string
	DateBucket    string
	PaymentID     string
	EventType     string
	Payload       string
	CreatedAt     time.Time
}

type Store struct {
	session *gocql.Session
}

func NewStore(hosts []string, keyspace string) (*Store, error) {
	cluster := gocql.NewCluster(hosts...)
	cluster.Keyspace = keyspace
	cluster.Consistency = gocql.LocalQuorum
	cluster.Timeout = 10 * time.Second

	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("connect scylla: %w", err)
	}
	return &Store{session: session}, nil
}

func (s *Store) Append(entry Entry) error {
	return s.session.Query(
		`INSERT INTO ledger (participant_id, date_bucket, payment_id, event_type, payload, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		entry.ParticipantID, entry.DateBucket, entry.PaymentID,
		entry.EventType, entry.Payload, entry.CreatedAt,
	).Exec()
}

func (s *Store) GetByParticipant(participantID, dateBucket string) ([]Entry, error) {
	iter := s.session.Query(
		`SELECT participant_id, date_bucket, payment_id, event_type, payload, created_at
		 FROM ledger WHERE participant_id = ? AND date_bucket = ?`,
		participantID, dateBucket,
	).Iter()

	var entries []Entry
	var entry Entry
	for iter.Scan(&entry.ParticipantID, &entry.DateBucket, &entry.PaymentID,
		&entry.EventType, &entry.Payload, &entry.CreatedAt) {
		entries = append(entries, entry)
	}
	return entries, iter.Close()
}

func (s *Store) Close() {
	s.session.Close()
}
