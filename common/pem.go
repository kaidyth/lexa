package common

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"log"
	"math/big"
	"os"
	"time"
)

func GenerateECKey(kFile *os.File) (key *ecdsa.PrivateKey) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Fatal("Failed to generate ECDSA key: %s\n")
		os.Exit(1)
	}

	keyDer, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		log.Fatal("Failed to generate ECDSA key: %s\n")
		os.Exit(1)
	}

	keyBlock := pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyDer,
	}

	pem.Encode(kFile, &keyBlock)

	return
}

func GenerateCertificate(pub, priv interface{}, cFile *os.File) {
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:         "LEXA DEFAULT CERTIFICATE",
			Organization:       []string{"Kaidyth"},
			OrganizationalUnit: []string{"Lexa"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 1),
	}

	certDer, err := x509.CreateCertificate(
		rand.Reader, &template, &template, pub, priv,
	)

	if err != nil {
		log.Fatal("Failed to generate self-signed certificate")
		os.Exit(1)
	}

	certBlock := pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certDer,
	}

	pem.Encode(cFile, &certBlock)
}
