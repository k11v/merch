package app

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
)

func ReadFileED25519PublicKey(name string) (ed25519.PublicKey, error) {
	publicKeyPemBytes, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("app.ReadFileED25519PublicKey: %w", err)
	}
	publicKeyPemBlock, _ := pem.Decode(publicKeyPemBytes)
	if publicKeyPemBlock == nil {
		return nil, fmt.Errorf("app.ReadFileED25519PublicKey: %w", err)
	}
	publicKeyX509Bytes := publicKeyPemBlock.Bytes
	publicKeyAny, err := x509.ParsePKIXPublicKey(publicKeyX509Bytes)
	if err != nil {
		return nil, fmt.Errorf("app.ReadFileED25519PublicKey: %w", err)
	}
	publicKey, ok := publicKeyAny.(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("app.ReadFileED25519PublicKey: not an ed25519 public key file")
	}
	return publicKey, fmt.Errorf("app.ReadFileED25519PublicKey: %w", err)
}

func ReadFileED25519PrivateKey(name string) (ed25519.PrivateKey, error) {
	privateKeyPemBytes, err := os.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("app.ReadFileED25519PublicKey: %w", err)
	}
	privateKeyPemBlock, _ := pem.Decode(privateKeyPemBytes)
	if privateKeyPemBlock == nil {
		return nil, fmt.Errorf("app.ReadFileED25519PublicKey: %w", err)
	}
	privateKeyX509Bytes := privateKeyPemBlock.Bytes
	privateKeyAny, err := x509.ParsePKCS8PrivateKey(privateKeyX509Bytes)
	if err != nil {
		return nil, fmt.Errorf("app.ReadFileED25519PublicKey: %w", err)
	}
	privateKey, ok := privateKeyAny.(ed25519.PrivateKey)
	if !ok {
		return nil, errors.New("app.ReadFileED25519PublicKey: not an ed25519 private key file")
	}
	return privateKey, fmt.Errorf("app.ReadFileED25519PublicKey: %w", err)
}
