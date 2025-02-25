package auth

import "testing"

func TestPasswordHasher(t *testing.T) {
	t.Run("hashes and verifies password", func(t *testing.T) {
		ph := NewPasswordHasher(DefaultArgon2IDParams())

		passwordHash, err := ph.Hash("password")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		err = ph.Verify("password", passwordHash)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
	})

	t.Run("verifies with different params", func(t *testing.T) {
		var (
			ph1 = NewPasswordHasher(&Argon2IDParams{
				Memory:      64 * 1024,
				Time:        1,
				Parallelism: 1,
				SaltLen:     16,
				HashLen:     32,
			})
			ph2 = NewPasswordHasher(&Argon2IDParams{
				Memory:      32 * 1024,
				Time:        2,
				Parallelism: 1,
				SaltLen:     16,
				HashLen:     32,
			})
		)

		passwordHash1, err := ph1.Hash("password")
		if err != nil {
			t.Fatalf("got %v error", err)
		}

		err = ph2.Verify("password", passwordHash1)
		if err != nil {
			t.Fatalf("got %v error", err)
		}
	})
}
