package participant_test

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"math/big"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"switch/internal/participant"
)

func testCert() *x509.Certificate {
	return &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(1 * time.Hour),
		Raw:          []byte("test-cert-raw-data"),
	}
}

func TestRegistry_RegisterAndResolve(t *testing.T) {
	r := participant.NewRegistry()
	cert := testCert()
	hash := sha256.Sum256(cert.Raw)
	thumbprint := hex.EncodeToString(hash[:])

	err := r.Register(&participant.Participant{
		ID: "bank-a", Name: "Bank A", BIC: "BANKUS33",
		Account: "ACC-123", CertHash: thumbprint,
	})
	require.NoError(t, err)

	pid, err := r.Resolve(context.Background(), cert)
	require.NoError(t, err)
	require.Equal(t, "bank-a", pid)
}

func TestRegistry_ResolveUnknownCert(t *testing.T) {
	r := participant.NewRegistry()
	cert := testCert()

	_, err := r.Resolve(context.Background(), cert)
	require.Error(t, err)
}

func TestRegistry_GetBankForParticipant(t *testing.T) {
	r := participant.NewRegistry()
	r.Register(&participant.Participant{
		ID: "bank-b", BIC: "BANKDEFF", Account: "ACC-456",
	})

	bic, acc, err := r.GetBankForParticipant(context.Background(), "bank-b")
	require.NoError(t, err)
	require.Equal(t, "BANKDEFF", bic)
	require.Equal(t, "ACC-456", acc)
}

func TestRegistry_GetBankForUnknownParticipant(t *testing.T) {
	r := participant.NewRegistry()
	_, _, err := r.GetBankForParticipant(context.Background(), "unknown")
	require.Error(t, err)
}

func TestRegistry_DuplicateRegistration(t *testing.T) {
	r := participant.NewRegistry()
	r.Register(&participant.Participant{ID: "dup"})
	err := r.Register(&participant.Participant{ID: "dup"})
	require.Error(t, err)
}
