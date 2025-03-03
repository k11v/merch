package app

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func SetupJWT(pubFile, privFile string) error {
	pubExists, err := fileExists(pubFile)
	if err != nil {
		return fmt.Errorf("app.SetupJWT: %w", err)
	}
	privExists, err := fileExists(privFile)
	if err != nil {
		return fmt.Errorf("app.SetupJWT: %w", err)
	}
	if pubExists || privExists {
		if pubExists && privExists {
			return nil
		}
		return errors.New("app.SetupJWT: only one of public and private key files exists")
	}

	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return fmt.Errorf("app.SetupJWT: %w", err)
	}
	pubPEM, err := pemEncodeED25519PublicKey(pub)
	if err != nil {
		return fmt.Errorf("app.SetupJWT: %w", err)
	}
	privPEM, err := pemEncodeED25519PrivateKey(priv)
	if err != nil {
		return fmt.Errorf("app.SetupJWT: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(pubFile), 0o700)
	if err != nil {
		return fmt.Errorf("app.SetupJWT: %w", err)
	}
	err = os.WriteFile(pubFile, pubPEM, 0o600)
	if err != nil {
		return fmt.Errorf("app.SetupJWT: %w", err)
	}

	err = os.MkdirAll(filepath.Dir(privFile), 0o700)
	if err != nil {
		return fmt.Errorf("app.SetupJWT: %w", err)
	}
	err = os.WriteFile(privFile, privPEM, 0o600)
	if err != nil {
		return fmt.Errorf("app.SetupJWT: %w", err)
	}

	return nil
}

func pemEncodeED25519PrivateKey(priv ed25519.PrivateKey) ([]byte, error) {
	privX509, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}
	privPEM := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privX509,
	}
	privPEMBuf := new(bytes.Buffer)
	err = pem.Encode(privPEMBuf, privPEM)
	if err != nil {
		return nil, err
	}
	return privPEMBuf.Bytes(), nil
}

func pemEncodeED25519PublicKey(pub ed25519.PublicKey) ([]byte, error) {
	pubX509, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, err
	}
	pubPEM := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubX509,
	}
	pubPEMBuf := new(bytes.Buffer)
	err = pem.Encode(pubPEMBuf, pubPEM)
	if err != nil {
		return nil, err
	}
	return pubPEMBuf.Bytes(), nil
}

func fileExists(name string) (bool, error) {
	switch _, err := os.Stat(name); {
	case err == nil:
		return true, nil
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, err
	}
}
