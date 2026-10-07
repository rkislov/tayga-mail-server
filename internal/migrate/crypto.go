package migrate

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ResolveKey returns a 32-byte AEAD key from explicit material or a persisted file.
func ResolveKey(explicit, dataDir string) ([]byte, error) {
	if s := strings.TrimSpace(explicit); s != "" {
		if raw, err := base64.StdEncoding.DecodeString(s); err == nil && len(raw) >= 16 {
			sum := sha256.Sum256(raw)
			return sum[:], nil
		}
		sum := sha256.Sum256([]byte(s))
		return sum[:], nil
	}
	if dataDir == "" {
		dataDir = "./data"
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, err
	}
	path := filepath.Join(dataDir, "secrets.key")
	if b, err := os.ReadFile(path); err == nil {
		raw := strings.TrimSpace(string(b))
		if dec, err := base64.StdEncoding.DecodeString(raw); err == nil && len(dec) >= 16 {
			sum := sha256.Sum256(dec)
			return sum[:], nil
		}
		sum := sha256.Sum256(b)
		return sum[:], nil
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, []byte(base64.StdEncoding.EncodeToString(raw)+"\n"), 0o600); err != nil {
		return nil, err
	}
	sum := sha256.Sum256(raw)
	return sum[:], nil
}

func seal(key, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("invalid key length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

func open(key, ciphertext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("invalid key length")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce, data := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	return gcm.Open(nil, nonce, data, nil)
}

func encryptPassword(master []byte, password string) ([]byte, error) {
	sum := sha256.Sum256(append(append([]byte{}, master...), []byte("tayga-migration-password-v1")...))
	return seal(sum[:], []byte(password))
}

func decryptPassword(master []byte, ciphertext []byte) (string, error) {
	if len(ciphertext) == 0 {
		return "", fmt.Errorf("no password")
	}
	sum := sha256.Sum256(append(append([]byte{}, master...), []byte("tayga-migration-password-v1")...))
	plain, err := open(sum[:], ciphertext)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}
