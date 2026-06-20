package crypto

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"time"
)

var ErrReplay = errors.New("replayed message")

type NonceStore interface {
	SetNX(ctx context.Context, key, val string, ttl time.Duration) (bool, error)
}

func VerifySignature(cert *x509.Certificate, canonicalXML, signature []byte) error {
	pub, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return errors.New("unsupported public key type")
	}
	digest := sha256.Sum256(canonicalXML)
	return rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], signature)
}

func CheckReplay(ctx context.Context, store NonceStore, participantID, nonce string, ts time.Time) error {
	if time.Since(ts) > 5*time.Minute || time.Until(ts) > time.Minute {
		return errors.New("timestamp out of window")
	}
	ok, err := store.SetNX(ctx, "nonce:"+participantID+":"+nonce, "1", 10*time.Minute)
	if err != nil {
		return err
	}
	if !ok {
		return ErrReplay
	}
	return nil
}
