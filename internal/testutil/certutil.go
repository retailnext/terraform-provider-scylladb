// Copyright RetailNext, Inc. 2026

package testutil

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"time"
)

type Cert struct {
	Cert       *x509.Certificate
	CertBytes  []byte
	PrivateKey *rsa.PrivateKey
}

type CertSubject struct {
	CommonName      string
	Organization    []string
	Country         []string
	DNSNames        []string
	IPAddresses     []net.IP
	DurationInYears int
}

func GenerateCert(caCert *Cert, subj CertSubject) (*Cert, error) {
	// set up certificate
	cert := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   subj.CommonName,
			Organization: subj.Organization,
			Country:      subj.Country,
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().AddDate(subj.DurationInYears, 0, 0),

		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		KeyUsage:    x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	if caCert == nil {
		cert.IsCA = true
		cert.BasicConstraintsValid = true // Must be true for IsCA to be recognized
		cert.KeyUsage = x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature
	} else {
		if len(subj.DNSNames) > 0 {
			cert.DNSNames = subj.DNSNames
		}
		if len(subj.IPAddresses) > 0 {
			cert.IPAddresses = subj.IPAddresses
		}
		cert.KeyUsage = x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment
	}

	// Generate private key
	privKey, err := rsa.GenerateKey(rand.Reader, 4096)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	var ca *x509.Certificate
	var caPrivKey *rsa.PrivateKey
	if caCert == nil {
		ca = cert
		caPrivKey = privKey
	} else {
		ca = caCert.Cert
		caPrivKey = caCert.PrivateKey
	}

	// Create certificate
	certBytes, err := x509.CreateCertificate(rand.Reader, cert, ca, &privKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to create certificate: %w", err)
	}
	return &Cert{
		Cert:       cert,
		CertBytes:  certBytes,
		PrivateKey: privKey,
	}, nil
}

func GenerateTestCACert() (*Cert, error) {
	return GenerateCert(nil, CertSubject{
		CommonName:      "My CA",
		Organization:    []string{"My Org, Inc."},
		Country:         []string{"US"},
		DurationInYears: 10,
	})
}

func GenerateTestServerCert(caCert *Cert) (*Cert, error) {
	return GenerateCert(caCert, CertSubject{
		CommonName:      "localhost",
		Organization:    []string{"My Org, Inc."},
		Country:         []string{"US"},
		DNSNames:        []string{"localhost"},
		IPAddresses:     []net.IP{net.IPv4(127, 0, 0, 1), net.IPv6loopback},
		DurationInYears: 1,
	})
}

func (c *Cert) PEMEncodedCert() (cert, privKey []byte, err error) {
	certBuffer := new(bytes.Buffer)
	err = pem.Encode(certBuffer, &pem.Block{Type: "CERTIFICATE", Bytes: c.CertBytes})
	if err != nil {
		return nil, nil, err
	}
	cert = certBuffer.Bytes()

	privKeyBuffer := new(bytes.Buffer)
	err = pem.Encode(privKeyBuffer, &pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(c.PrivateKey),
	})
	if err != nil {
		return nil, nil, err
	}
	privKey = privKeyBuffer.Bytes()

	return cert, privKey, nil
}

func (c *Cert) SaveToFiles(certFilePath, keyFilePath string) error {
	certPEM, keyPEM, err := c.PEMEncodedCert()
	if err != nil {
		return err
	}
	if err := os.WriteFile(certFilePath, certPEM, 0644); err != nil {
		return fmt.Errorf("failed to write cert file: %w", err)
	}
	if err := os.WriteFile(keyFilePath, keyPEM, 0600); err != nil {
		return fmt.Errorf("failed to write key file: %w", err)
	}
	return nil
}
