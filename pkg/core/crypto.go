package core

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// EncryptedPrefix defines the wire prefix for all encrypted credential fields.
const EncryptedPrefix = "enc:v1:"

const (
	pbkdf2Rounds = 100000
	keyLen       = 32 // 256 bits
)

// pbkdf2SHA256 implements RFC 2898 standard PBKDF2 with HMAC-SHA256 using standard library.
func pbkdf2SHA256(password, salt []byte, iter, keyLen int) []byte {
	mac := hmac.New(sha256.New, password)
	hashLen := mac.Size()
	numBlocks := (keyLen + hashLen - 1) / hashLen
	var dk []byte

	for block := 1; block <= numBlocks; block++ {
		mac.Reset()
		mac.Write(salt)
		var b [4]byte
		b[0] = byte(block >> 24)
		b[1] = byte(block >> 16)
		b[2] = byte(block >> 8)
		b[3] = byte(block)
		mac.Write(b[:])
		u := mac.Sum(nil)

		t := make([]byte, len(u))
		copy(t, u)

		for i := 2; i <= iter; i++ {
			mac.Reset()
			mac.Write(u)
			u = mac.Sum(nil)
			for j := range t {
				t[j] ^= u[j]
			}
		}
		dk = append(dk, t...)
	}
	return dk[:keyLen]
}

// GetDefaultVaultKey retrieves or creates the local vault encryption key (~/.config/antigravity-swiss/.vault_key).
func GetDefaultVaultKey() []byte {
	configDir := GetConfigDir()
	_ = os.MkdirAll(configDir, 0700)
	keyFile := filepath.Join(configDir, ".vault_key")

	if data, err := os.ReadFile(keyFile); err == nil && len(data) == keyLen {
		return data
	}

	newKey := make([]byte, keyLen)
	if _, err := io.ReadFull(rand.Reader, newKey); err != nil {
		h := sha256.Sum256([]byte(configDir + ":antigravity-swiss-vault"))
		return h[:]
	}

	_ = os.WriteFile(keyFile, newKey, 0600)
	return newKey
}

// EncryptCredential encrypts sensitive text with AES-256-GCM.
func EncryptCredential(plaintext string) string {
	if plaintext == "" || strings.HasPrefix(plaintext, EncryptedPrefix) {
		return plaintext
	}

	key := GetDefaultVaultKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return plaintext
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return plaintext
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return plaintext
	}

	// gcm.Seal appends ciphertext + tag to nonce
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return EncryptedPrefix + base64.StdEncoding.EncodeToString(sealed)
}

// DecryptCredential decrypts an encrypted credential with AES-256-GCM.
func DecryptCredential(ciphertext string) string {
	if ciphertext == "" || !strings.HasPrefix(ciphertext, EncryptedPrefix) {
		return ciphertext
	}

	rawB64 := strings.TrimPrefix(ciphertext, EncryptedPrefix)
	packed, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil || len(packed) < 28 {
		return ""
	}

	key := GetDefaultVaultKey()
	block, err := aes.NewCipher(key)
	if err != nil {
		return ""
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return ""
	}

	nonceSize := gcm.NonceSize()
	if len(packed) < nonceSize {
		return ""
	}

	nonce, encryptedData := packed[:nonceSize], packed[nonceSize:]
	decrypted, err := gcm.Open(nil, nonce, encryptedData, nil)
	if err != nil {
		return ""
	}

	return string(decrypted)
}

// ValidateAppPassword checks if the proposed password meets the >= 6 character requirement.
func ValidateAppPassword(password string) (bool, string) {
	if strings.TrimSpace(password) == "" {
		return false, "Password cannot be empty."
	}
	if len(password) < 6 {
		return false, "Password must be at least 6 characters long."
	}
	return true, ""
}

// HashAppPassword generates a PBKDF2-HMAC-SHA256 hash with 16-byte random salt.
// Format: pbkdf2:sha256:100000:<salt_hex>:<hash_hex>
func HashAppPassword(password string) string {
	salt := make([]byte, 16)
	_, _ = io.ReadFull(rand.Reader, salt)

	dk := pbkdf2SHA256([]byte(password), salt, pbkdf2Rounds, 32)
	return fmt.Sprintf("pbkdf2:sha256:%d:%s:%s", pbkdf2Rounds, hex.EncodeToString(salt), hex.EncodeToString(dk))
}

// VerifyAppPassword verifies a password against the stored PBKDF2 hash.
func VerifyAppPassword(password, hashed string) bool {
	if password == "" || hashed == "" {
		return false
	}

	parts := strings.Split(hashed, ":")
	if len(parts) != 5 || parts[0] != "pbkdf2" || parts[1] != "sha256" {
		return false
	}

	rounds, err := strconv.Atoi(parts[2])
	if err != nil {
		return false
	}

	salt, err := hex.DecodeString(parts[3])
	if err != nil {
		return false
	}

	expectedDK, err := hex.DecodeString(parts[4])
	if err != nil {
		return false
	}

	actualDK := pbkdf2SHA256([]byte(password), salt, rounds, len(expectedDK))
	return subtle.ConstantTimeCompare(actualDK, expectedDK) == 1
}
