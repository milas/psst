package main

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/grantae/certinfo"
	v1 "k8s.io/api/core/v1"
)

const SecretTypeTLS = "kubernetes.io/tls"

func FormatSecretTLS(_ context.Context, s *v1.Secret) (string, error) {
	if s.Type != SecretTypeTLS {
		return "", fmt.Errorf("invalid secret type: %v", s.Type)
	}

	tlsData, ok := s.Data["tls.crt"]
	if !ok {
		return "", errors.New("secret missing key: tls.crt")
	} else if len(tlsData) == 0 {
		return "", errors.New("secret has empty key: tls.crt")
	}

	var msgs []string
	for block, rest := pem.Decode(tlsData); block != nil; block, rest = pem.Decode(rest) {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("parsing TLS public key: %v", err)
		}
		msg, err := certinfo.CertificateText(cert)
		if err != nil {
			return "", fmt.Errorf("formatting TLS public key: %v", err)
		}
		msgs = append(msgs, msg)
	}

	if len(msgs) == 0 {
		return "", fmt.Errorf("invalid PEM data: %v", string(tlsData))
	}

	return strings.Join(msgs, "\n"), nil
}
