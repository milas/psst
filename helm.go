package main

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"

	v1 "k8s.io/api/core/v1"
)

const SecretTypeHelm = "helm.sh/release.v1"

func FormatHelmSecret(s *v1.Secret) (string, error) {
	if s.Type != SecretTypeHelm {
		return "", fmt.Errorf("invalid secret type: %v", s.Type)
	}
	d, ok := s.Data["release"]
	if !ok {
		return "", fmt.Errorf("secret missing key: release")
	}
	gzipData := make([]byte, base64.StdEncoding.DecodedLen(len(d)))
	if n, err := base64.StdEncoding.Decode(gzipData, d); err != nil {
		return "", fmt.Errorf("base64 decoding release data: %w", err)
	} else {
		gzipData = gzipData[:n]
	}

	gr, err := gzip.NewReader(bytes.NewReader(gzipData))
	if err != nil {
		return "", fmt.Errorf("gzip decompressing release data: %w", err)
	}

	dec := json.NewDecoder(gr)

	release := make(map[string]interface{})
	if err := dec.Decode(&release); err != nil {
		return "", fmt.Errorf("json unmarshaling release data: %w", err)
	}

	msgBytes, err := json.MarshalIndent(release, "", "  ")
	if err != nil {
		return "", fmt.Errorf("preparing release output: %w", err)
	}
	return string(msgBytes), nil
}
