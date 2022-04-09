package cert

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"math/big"
	"time"
)

type PemCertPair struct {
	Cert []byte
	Key  []byte
}

type CertConfig struct {
	Parent     *PemCertPair
	IsServer   bool
	IsClient   bool
	Serial     *big.Int
	CommonName string
	DNSNames   []string
	SecsValid  uint
}

// Reference: https://shaneutt.com/blog/golang-ca-and-signed-cert-go/
func MakeCerts(config CertConfig) (*PemCertPair, error) {
	var extKeyUsage []x509.ExtKeyUsage
	var keyUsage x509.KeyUsage
	var ok bool

	if config.IsClient {
		extKeyUsage = append(extKeyUsage, x509.ExtKeyUsageClientAuth)
		keyUsage |= x509.KeyUsageDigitalSignature
	}

	if config.IsServer {
		extKeyUsage = append(extKeyUsage, x509.ExtKeyUsageServerAuth)
		keyUsage |= x509.KeyUsageDigitalSignature
	}

	if config.Parent == nil {
		extKeyUsage = append(extKeyUsage, x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth)
		keyUsage |= x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature | x509.KeyUsageCRLSign
	}

	cert := &x509.Certificate{
		SerialNumber: config.Serial,
		Subject: pkix.Name{
			Country:      []string{"🇺🇸"},
			Province:     []string{"Pennsylvania"},
			Locality:     []string{"Pittsburgh"},
			Organization: []string{"PlaidCTF"},
			CommonName:   config.CommonName,
		},
		DNSNames:              config.DNSNames,
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Second * time.Duration(config.SecsValid)),
		IsCA:                  config.Parent == nil,
		KeyUsage:              keyUsage,
		ExtKeyUsage:           extKeyUsage,
		BasicConstraintsValid: true,
	}

	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	parent := cert
	parentKey := privKey
	if config.Parent != nil {
		block, _ := pem.Decode(config.Parent.Cert)
		parent, err = x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}

		block, _ = pem.Decode(config.Parent.Key)
		parsedKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		parentKey, ok = parsedKey.(ed25519.PrivateKey)
		if !ok {
			return nil, errors.New("key of signer is incorrect type")
		}
	}
	rawCert, err := x509.CreateCertificate(rand.Reader, cert, parent, pubKey, parentKey)
	if err != nil {
		return nil, err
	}

	pemCert := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: rawCert,
	})

	rawPrivKey, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return nil, err
	}

	pemKey := pem.EncodeToMemory(&pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: rawPrivKey,
	})

	return &PemCertPair{
		Cert: pemCert,
		Key:  pemKey,
	}, nil
}
