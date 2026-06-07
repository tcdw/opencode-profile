package transfer

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/pbkdf2"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

// Secrets blob parameters. The format is intentionally self-describing so a
// bundle can be decrypted without consulting the manifest:
//
//	magic("OCPS") | ver(1B) | salt(16B) | nonce(12B) | AES-256-GCM ciphertext
//
// The 5-byte magic+version prefix is the AEAD additional data, so flipping the
// version or magic also fails authentication.
const (
	kdfName   = "pbkdf2-sha256"
	kdfIter   = 600_000
	saltLen   = 16
	nonceLen  = 12
	keyLen    = 32 // AES-256
	cryptoVer = 1
)

var cryptoMagic = []byte("OCPS")

// Seal encrypts plain under a key derived from pass and returns the blob above.
func Seal(plain []byte, pass string) ([]byte, error) {
	if pass == "" {
		return nil, errors.New("empty passphrase")
	}
	salt := make([]byte, saltLen)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	nonce := make([]byte, nonceLen)
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	gcm, err := newGCM(pass, salt)
	if err != nil {
		return nil, err
	}
	header := blobHeader(salt, nonce)
	aad := header[:len(cryptoMagic)+1]
	ct := gcm.Seal(nil, nonce, plain, aad)
	return append(header, ct...), nil
}

// Open reverses Seal. A wrong passphrase or any tampering fails authentication.
func Open(blob []byte, pass string) ([]byte, error) {
	if pass == "" {
		return nil, errors.New("empty passphrase")
	}
	hdr := len(cryptoMagic) + 1
	if len(blob) < hdr+saltLen+nonceLen {
		return nil, errors.New("secrets blob too short or corrupt")
	}
	if !bytes.Equal(blob[:len(cryptoMagic)], cryptoMagic) {
		return nil, errors.New("not an ocp secrets blob")
	}
	if ver := blob[len(cryptoMagic)]; ver != cryptoVer {
		return nil, fmt.Errorf("unsupported secrets version %d", ver)
	}
	off := hdr
	salt := blob[off : off+saltLen]
	off += saltLen
	nonce := blob[off : off+nonceLen]
	off += nonceLen
	ct := blob[off:]
	gcm, err := newGCM(pass, salt)
	if err != nil {
		return nil, err
	}
	plain, err := gcm.Open(nil, nonce, ct, blob[:hdr])
	if err != nil {
		return nil, errors.New("decryption failed: wrong passphrase or corrupt bundle")
	}
	return plain, nil
}

func newGCM(pass string, salt []byte) (cipher.AEAD, error) {
	key, err := pbkdf2.Key(sha256.New, pass, salt, kdfIter, keyLen)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func blobHeader(salt, nonce []byte) []byte {
	h := make([]byte, 0, len(cryptoMagic)+1+len(salt)+len(nonce))
	h = append(h, cryptoMagic...)
	h = append(h, cryptoVer)
	h = append(h, salt...)
	h = append(h, nonce...)
	return h
}
