package main

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	v1 "k8s.io/api/core/v1"
)

const SecretTypeTLS = "kubernetes.io/tls"

func boolStatus(v bool) string {
	if v {
		return "✔️"
	}
	return "❌"
}

func FormatSecretTLS(_ context.Context, secret *v1.Secret) (string, error) {
	if secret.Type != SecretTypeTLS {
		return "", fmt.Errorf("invalid secret type: %v", secret.Type)
	}

	now := time.Now()
	tlsData, ok := secret.Data["tls.crt"]
	if !ok {
		return "", errors.New("secret missing key: tls.crt")
	} else if len(tlsData) == 0 {
		return "", errors.New("secret has empty key: tls.crt")
	}

	var msgs []string
	var leaf *x509.Certificate
	intermediates := x509.NewCertPool()

	for block, rest := pem.Decode(tlsData); block != nil; block, rest = pem.Decode(rest) {
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("parsing TLS public key: %v", err)
		}
		if leaf == nil {
			leaf = cert
		} else {
			intermediates.AddCert(cert)
		}
	}

	cols := []table.Column{
		{Width: 14},
		{Title: leaf.Subject.CommonName, Width: 96},
	}

	const timestampFormat = "02 Jan 2006 15:04:05 MST"
	rows := []table.Row{
		{"Subject", leaf.Subject.String()},
		{"Issuer", leaf.Issuer.String()},
		{"Not Before", leaf.NotBefore.Format(timestampFormat) + " " + boolStatus(leaf.NotBefore.Before(now))},
		{"Not After", leaf.NotAfter.Format(timestampFormat) + " " + boolStatus(leaf.NotAfter.After(now))},
		{"Algorithm", leaf.PublicKeyAlgorithm.String()},
	}

	switch leaf.PublicKeyAlgorithm {
	case x509.RSA:
		k := leaf.PublicKey.(*rsa.PublicKey)
		rows = append(rows, table.Row{"Key Size", fmt.Sprintf("%d-bit", k.N.BitLen())})
	case x509.ECDSA:
		k := leaf.PublicKey.(*ecdsa.PublicKey)
		rows = append(rows, table.Row{"Key Size", fmt.Sprintf("%d-bit", k.Params().BitSize)})
	}

	if len(leaf.SubjectKeyId) != 0 {
		rows = append(rows, table.Row{"Subject Key ID", formatFingerprint(leaf.SubjectKeyId)})
	}

	verifyOpts := x509.VerifyOptions{
		DNSName:       secret.Annotations["leaf-manager.io/common-name"],
		Intermediates: intermediates,
		CurrentTime:   now,
	}
	rows = append(rows, trust(leaf, verifyOpts)...)

	sha1Fingerprint := sha1.Sum(leaf.Raw)
	sha256Fingerprint := sha256.Sum256(leaf.Raw)
	rows = append(
		rows,
		table.Row{},
		table.Row{"Fingerprints"},
		table.Row{"  SHA-1", formatFingerprint(sha1Fingerprint[:])},
		table.Row{"  SHA-256", formatFingerprint(sha256Fingerprint[:])},
	)

	if sanRows := subjAltNames(leaf); len(sanRows) != 0 {
		rows = append(rows, table.Row{}, table.Row{"SAN"})
		for i := range sanRows {
			sanRows[i][0] = "  " + sanRows[i][0]
		}
		rows = append(rows, sanRows...)
	}

	t := table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithHeight(len(rows)),
	)

	s := table.Styles{
		Header: lipgloss.NewStyle().
			Bold(true).
			Padding(0, 1).
			MarginTop(1).
			Foreground(lipgloss.Color("231")).
			Background(lipgloss.Color("63")).
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			BorderBottom(true),
		Cell: lipgloss.NewStyle().
			Padding(0, 1),
	}
	t.SetStyles(s)

	msgs = append(msgs, t.View())
	return strings.Join(msgs, "\n"), nil
}

func trust(leaf *x509.Certificate, opts x509.VerifyOptions) []table.Row {
	const trustLabel = "System Trust"
	chains, err := leaf.Verify(opts)
	if err != nil {
		return []table.Row{{trustLabel, "❗ " + err.Error()}}
	}

	if len(chains) < 1 {
		return []table.Row{{trustLabel, "🔒 System"}}
	}

	chainPaths := make([]string, len(chains))
	for i := range chains {
		// first entry is always the leaf itself
		chain := chains[i][1:]

		names := make([]string, len(chain))
		for i := range chain {
			var name string
			if orgs := chain[i].Subject.Organization; len(orgs) != 0 {
				name = orgs[0] + "/"
			}
			name += chain[i].Subject.CommonName
			names[i] = name
		}
		chainPaths[i] = strings.Join(names, " -> ")
	}

	if len(chainPaths) == 1 {
		return []table.Row{{trustLabel, "🔒 " + chainPaths[0]}}
	}

	rows := make([]table.Row, len(chainPaths)+1)
	rows[0] = table.Row{trustLabel, "🔒"}
	for i := range chainPaths {
		rows[i] = table.Row{"", chainPaths[i]}
	}

	return rows
}

func formatFingerprint(d []byte) string {
	var buf bytes.Buffer
	for i, f := range d {
		if i > 0 {
			fmt.Fprintf(&buf, ":")
		}
		fmt.Fprintf(&buf, "%02X", f)
	}
	return buf.String()
}

func subjAltNames(cert *x509.Certificate) []table.Row {
	var rows []table.Row
	if len(cert.DNSNames) != 0 {
		rows = append(rows, table.Row{"DNS", strings.Join(cert.DNSNames, ", ")})
	}
	if len(cert.IPAddresses) != 0 {
		ipAddrs := make([]string, len(cert.IPAddresses))
		for i := range cert.IPAddresses {
			ipAddrs[i] = cert.IPAddresses[i].String()
		}
		rows = append(rows, table.Row{"IP", strings.Join(ipAddrs, ", ")})
	}
	if len(cert.EmailAddresses) != 0 {
		rows = append(rows, table.Row{"E-mail", strings.Join(cert.EmailAddresses, ", ")})
	}
	if len(cert.URIs) != 0 {
		uris := make([]string, len(cert.URIs))
		for i := range cert.URIs {
			uris[i] = cert.URIs[i].String()
		}
		rows = append(rows, table.Row{"URI", strings.Join(uris, ", ")})
	}
	return rows
}
