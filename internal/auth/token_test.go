package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"testing"

	"github.com/google/uuid"
)

func TestToken(t *testing.T) {
	t.Run("issues and authenticates token", func(t *testing.T) {
		var (
			pub, priv = generateTestED25519Key(t)
			ti        = NewTokenIssuer(priv)
			ta        = NewTokenAuthenticator(pub)
			userID    = uuid.MustParse("10000000-0000-0000-0000-000000000000")
		)

		token, err := ti.IssueToken(userID)
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		data, err := ta.AuthenticateToken(token)
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		if got, want := data.UserID, userID; got != want {
			t.Fatalf("got %v user ID, want %v", got, want)
		}
	})
}

func generateTestED25519Key(t *testing.T) (ed25519.PublicKey, ed25519.PrivateKey) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("can't generate ed25519 key: %v", err)
	}
	return pub, priv
}
