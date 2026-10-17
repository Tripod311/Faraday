package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/crypto/argon2"
)

const (
	memory      uint32 = 64 * 1024 // KB
	iterations  uint32 = 3
	parallelism uint8  = 4

	maxPlaintext = 8 << 20  // 8 MB
	maxFileSize  = 16 << 20 // 16 MB
)

var ErrAuthentication = errors.New(
	"invalid password, or vault damaged",
)

type kdfParams struct {
	Algorithm   string `json:"algorithm"`
	Salt        []byte `json:"salt"`
	Memory      uint32 `json:"memory"`
	Iterations  uint32 `json:"iterations"`
	Parallelism uint8  `json:"parallelism"`
}

type cipherParams struct {
	Algorithm string `json:"algorithm"`
	Nonce     []byte `json:"nonce"`
}

type header struct {
	Version int          `json:"version"`
	KDF     kdfParams    `json:"kdf"`
	Cipher  cipherParams `json:"cipher"`
}

type envelope struct {
	header
	Ciphertext []byte `json:"ciphertext"`
}

func CreateVault(filename string, password []byte) error {
	data, err := encrypt(password, json.RawMessage(`[]`))
	if err != nil {
		return err
	}

	f, err := os.OpenFile(
		filename,
		os.O_WRONLY|os.O_CREATE|os.O_EXCL,
		0600,
	)
	if err != nil {
		return err
	}

	ok := false
	defer func() {
		f.Close()
		if !ok {
			os.Remove(filename)
		}
	}()

	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	ok = true
	return nil
}

func WriteVault(
	filename string,
	password []byte,
	data json.RawMessage,
	newPassword ...[]byte,
) error {
	if len(newPassword) > 1 {
		return errors.New("only one new password is allowed")
	}
	if !json.Valid(data) {
		return errors.New("content is not a JSON")
	}
	if len(data) > maxPlaintext {
		return errors.New("content exceeding limit (8 MB max)")
	}

	encryptionPassword := password
	if len(newPassword) == 1 {
		if len(newPassword[0]) == 0 {
			return errors.New("new password must not be empty")
		}
		encryptionPassword = newPassword[0]
	}

	previous, err := ReadVault(filename, password)
	if err != nil {
		return err
	}
	clear(previous)

	encrypted, err := encrypt(encryptionPassword, data)
	if err != nil {
		return err
	}

	return replaceFile(filename, encrypted)
}

func ReadVault(
	filename string,
	password []byte,
) (json.RawMessage, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	data, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFileSize {
		return nil, errors.New("vault exceeding the limit (8 MB max)")
	}

	var env envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, fmt.Errorf("invalid vault format: %w", err)
	}

	// check parameters before launching argon
	// 1 fixed profile in first version
	if env.Version != 1 ||
		env.KDF.Algorithm != "argon2id" ||
		env.KDF.Memory != memory ||
		env.KDF.Iterations != iterations ||
		env.KDF.Parallelism != parallelism ||
		len(env.KDF.Salt) != 16 ||
		env.Cipher.Algorithm != "aes-256-gcm" ||
		len(env.Cipher.Nonce) != 12 {
		return nil, errors.New(
			"unsupported profile or vault damaged",
		)
	}

	gcm, err := makeGCM(password, env.KDF)
	if err != nil {
		return nil, err
	}

	aad, err := json.Marshal(env.header)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(
		nil,
		env.Cipher.Nonce,
		env.Ciphertext,
		aad,
	)
	if err != nil {
		return nil, ErrAuthentication
	}

	if len(plaintext) > maxPlaintext || !json.Valid(plaintext) {
		clear(plaintext)
		return nil, errors.New("vault contents is not a valid JSON")
	}

	return json.RawMessage(plaintext), nil
}

func encrypt(
	password []byte,
	data json.RawMessage,
) ([]byte, error) {
	if len(password) == 0 {
		return nil, errors.New("password can't be empty")
	}
	if len(data) > maxPlaintext || !json.Valid(data) {
		return nil, errors.New("недопустимый JSON или превышен лимит 8 МиБ")
	}

	h := header{
		Version: 1,
		KDF: kdfParams{
			Algorithm:   "argon2id",
			Salt:        make([]byte, 16),
			Memory:      memory,
			Iterations:  iterations,
			Parallelism: parallelism,
		},
		Cipher: cipherParams{
			Algorithm: "aes-256-gcm",
			Nonce:     make([]byte, 12),
		},
	}

	if _, err := rand.Read(h.KDF.Salt); err != nil {
		return nil, err
	}
	if _, err := rand.Read(h.Cipher.Nonce); err != nil {
		return nil, err
	}

	gcm, err := makeGCM(password, h.KDF)
	if err != nil {
		return nil, err
	}

	aad, err := json.Marshal(h)
	if err != nil {
		return nil, err
	}

	env := envelope{
		header: h,
		Ciphertext: gcm.Seal(
			nil,
			h.Cipher.Nonce,
			data,
			aad,
		),
	}

	return json.MarshalIndent(env, "", "  ")
}

func makeGCM(password []byte, p kdfParams) (cipher.AEAD, error) {
	key := argon2.IDKey(
		password,
		p.Salt,
		p.Iterations,
		p.Memory,
		p.Parallelism,
		32,
	)
	defer clear(key)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}

// Write temporary then replace
func replaceFile(filename string, data []byte) error {
	f, err := os.CreateTemp(
		filepath.Dir(filename),
		".faraday-*.tmp",
	)
	if err != nil {
		return err
	}

	tmp := f.Name()
	defer os.Remove(tmp)
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	return os.Rename(tmp, filename)
}
