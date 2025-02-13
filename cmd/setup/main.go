package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"github.com/k11v/merch/internal/app"
)

func main() {
	const envPostgresURL = "APP_POSTGRES_URL"
	postgresURL := os.Getenv(envPostgresURL)
	if postgresURL == "" {
		err := fmt.Errorf("%s env is empty", envPostgresURL)
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	const envJWTVerificationKeyFile = "APP_JWT_VERIFICATION_KEY_FILE"
	jwtVerificationKeyFile := os.Getenv(envJWTVerificationKeyFile)
	if jwtVerificationKeyFile == "" {
		err := fmt.Errorf("%s env is empty", envJWTVerificationKeyFile)
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	const envJWTSignatureKeyFile = "APP_JWT_SIGNATURE_KEY_FILE"
	jwtSignatureKeyFile := os.Getenv(envJWTSignatureKeyFile)
	if jwtSignatureKeyFile == "" {
		err := fmt.Errorf("%s env is empty", envJWTSignatureKeyFile)
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	err := app.SetupPostgres(postgresURL)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	err = SetupJWT(jwtVerificationKeyFile, jwtSignatureKeyFile)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func SetupJWT(verificationKeyFile, signatureKeyFile string) error {
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		return err
	}

	pubX509, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return err
	}
	pubPem := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubX509,
	}
	pubPemBuf := new(bytes.Buffer)
	err = pem.Encode(pubPemBuf, pubPem)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(verificationKeyFile), 0o700)
	if err != nil {
		return err
	}
	err = os.WriteFile(verificationKeyFile, pubPemBuf.Bytes(), 0o600)
	if err != nil {
		return err
	}

	privX509, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return err
	}
	privPem := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: privX509,
	}
	privPemBuf := new(bytes.Buffer)
	err = pem.Encode(privPemBuf, privPem)
	if err != nil {
		return err
	}
	err = os.MkdirAll(filepath.Dir(signatureKeyFile), 0o700)
	if err != nil {
		return err
	}
	err = os.WriteFile(signatureKeyFile, privPemBuf.Bytes(), 0o600)
	if err != nil {
		return err
	}

	return nil
}
