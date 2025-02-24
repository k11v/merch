package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"golang.org/x/crypto/argon2"
)

type Argon2IDParams struct {
	Memory      uint32
	Time        uint32
	Parallelism uint8
	SaltLen     uint32
	HashLen     uint32
}

func DefaultArgon2IDParams() *Argon2IDParams {
	return &Argon2IDParams{
		Memory:      1,
		Time:        64 * 1024,
		Parallelism: 1,
		SaltLen:     16,
		HashLen:     32,
	}
}

type PasswordHasher struct {
	argon2IDParams *Argon2IDParams
}

func NewPasswordHasher(argon2IDParams *Argon2IDParams) *PasswordHasher {
	return &PasswordHasher{argon2IDParams: argon2IDParams}
}

// Hash derives an Argon2ID hash from the password
// and returns it in the PHC string format.
// See https://github.com/P-H-C/phc-string-format/blob/master/phc-sf-spec.md.
func (ph *PasswordHasher) Hash(password string) (string, error) {
	salt := make([]byte, ph.argon2IDParams.SaltLen)
	_, err := rand.Read(salt)
	if err != nil {
		return "", fmt.Errorf("HashPasswordArgon2ID: %w", err)
	}
	hashRaw := argon2.IDKey(
		[]byte(password),
		salt,
		ph.argon2IDParams.Time,
		ph.argon2IDParams.Memory,
		ph.argon2IDParams.Parallelism,
		ph.argon2IDParams.HashLen,
	)
	hashEnc := formatPasswordHashArgon2ID(hashRaw, salt, ph.argon2IDParams)
	return hashEnc, nil
}

// Verify checks that the password matches the Argon2ID hash
// provided in the PHC string format.
func (ph *PasswordHasher) Verify(password, passwordHash string) error {
	wantHash, salt, params, err := parsePasswordHashArgon2ID(passwordHash)
	if err != nil {
		return fmt.Errorf("VerifyPasswordArgon2ID: %w", err)
	}
	gotHash := argon2.IDKey(
		[]byte(password),
		salt,
		params.Time,
		params.Memory,
		params.Parallelism,
		params.HashLen,
	)
	if subtle.ConstantTimeCompare(gotHash, wantHash) == 1 {
		return nil
	}
	return fmt.Errorf("VerifyPasswordArgon2ID: %w", ErrPasswordNotMatch)
}

func parsePasswordHashArgon2ID(passwordHash string) (hash []byte, salt []byte, params *Argon2IDParams, err error) {
	fieldsString, found := strings.CutPrefix(passwordHash, "$")
	if !found {
		return nil, nil, nil, errors.New("invalid prefix")
	}
	fields := strings.Split(fieldsString, "$")
	if len(fields) != 5 {
		return nil, nil, nil, errors.New("invalid fields count")
	}

	idField := fields[0]
	if idField != "argon2id" {
		return nil, nil, nil, errors.New("unsupported id")
	}

	versionField := fields[1]
	if versionField != "v="+strconv.Itoa(argon2.Version) {
		return nil, nil, nil, errors.New("unsupported version")
	}

	paramsFieldReader := strings.NewReader(fields[2])
	var (
		m uint32
		t uint32
		p uint8
	)
	_, err = fmt.Fscanf(paramsFieldReader, "m=%d,t=%d,p=%d", &m, &t, &p)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("params: %w", err)
	}
	if paramsFieldReader.Len() != 0 {
		return nil, nil, nil, errors.New("params: extra string")
	}

	saltField := fields[3]
	salt, err = base64.RawStdEncoding.DecodeString(saltField)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("salt: %w", err)
	}
	saltLen, err := intToUint32(len(salt))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("salt: %w", err)
	}

	hashField := fields[4]
	hash, err = base64.RawStdEncoding.DecodeString(hashField)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("hash: %w", err)
	}
	hashLen, err := intToUint32(len(hash))
	if err != nil {
		return nil, nil, nil, fmt.Errorf("hash: %w", err)
	}

	params = &Argon2IDParams{
		Memory:      m,
		Time:        t,
		Parallelism: p,
		SaltLen:     saltLen,
		HashLen:     hashLen,
	}

	return hash, salt, params, nil
}

func formatPasswordHashArgon2ID(hash []byte, salt []byte, params *Argon2IDParams) string {
	saltB64 := base64.RawStdEncoding.EncodeToString(salt)
	hashB64 := base64.RawStdEncoding.EncodeToString(hash)

	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		params.Memory,
		params.Time,
		params.Parallelism,
		saltB64,
		hashB64,
	)
}

func intToUint32(i int) (uint32, error) {
	if i < 0 || i > math.MaxUint32 {
		return 0, errors.New("int out of uint32 bounds")
	}
	return uint32(i), nil
}
