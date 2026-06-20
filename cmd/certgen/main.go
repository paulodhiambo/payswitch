package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"net"
	"os"
	"time"
)

func main() {
	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal(err)
	}

	caTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Switch Dev CA"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		log.Fatal(err)
	}

	writePEM("ca-cert.pem", "CERTIFICATE", caDER)
	writePEM("ca-key.pem", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(caKey))

	serverKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal(err)
	}

	serverTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject: pkix.Name{
			Organization: []string{"Switch Dev Server"},
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:    []string{"localhost", "gateway", "*.payment-switch.svc"},
		IPAddresses: []net.IP{net.ParseIP("127.0.0.1")},
	}

	caCert, err := x509.ParseCertificate(caDER)
	if err != nil {
		log.Fatal(err)
	}

	serverDER, err := x509.CreateCertificate(rand.Reader, serverTmpl, caCert, &serverKey.PublicKey, caKey)
	if err != nil {
		log.Fatal(err)
	}

	writePEM("server-cert.pem", "CERTIFICATE", serverDER)
	writePEM("server-key.pem", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(serverKey))

	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		log.Fatal(err)
	}

	clientTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject: pkix.Name{
			Organization: []string{"Switch Dev Client bank-a"},
			CommonName:   "bank-a",
		},
		NotBefore:   time.Now(),
		NotAfter:    time.Now().Add(10 * 365 * 24 * time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}

	clientDER, err := x509.CreateCertificate(rand.Reader, clientTmpl, caCert, &clientKey.PublicKey, caKey)
	if err != nil {
		log.Fatal(err)
	}

	writePEM("client-bank-a-cert.pem", "CERTIFICATE", clientDER)
	writePEM("client-bank-a-key.pem", "RSA PRIVATE KEY", x509.MarshalPKCS1PrivateKey(clientKey))

	log.Print("generated dev certs: ca-cert.pem, ca-key.pem, server-cert.pem, server-key.pem, client-bank-a-*.pem")
}

func writePEM(path, kind string, data []byte) {
	f, err := os.Create(path)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	pem.Encode(f, &pem.Block{Type: kind, Bytes: data})
}
