package crypto

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/binary"
	"fmt"
	"io"

	"filippo.io/edwards25519"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/nacl/secretbox"
)

// hkdfSalt is a fixed protocol salt for HKDF derivations.
var hkdfSalt = []byte("cgram-hkdf-salt-v1")

const (
	NonceSize = 24
	KeySize   = 32
)

type KeyPair struct {
	Private []byte
	Public  []byte
}

// GenerateEd25519 creates an Ed25519 signing key pair.
func GenerateEd25519() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate ed25519: %w", err)
	}
	return &KeyPair{Private: priv, Public: pub}, nil
}

// GenerateX25519 creates an X25519 key pair for Diffie-Hellman.
func GenerateX25519() (*KeyPair, error) {
	var priv [KeySize]byte
	if _, err := rand.Read(priv[:]); err != nil {
		return nil, fmt.Errorf("generate x25519 private: %w", err)
	}
	clampPrivateKey(&priv)

	pub, err := curve25519.X25519(priv[:], curve25519.Basepoint)
	if err != nil {
		return nil, fmt.Errorf("generate x25519 public: %w", err)
	}

	return &KeyPair{Private: priv[:], Public: pub}, nil
}

func clampPrivateKey(key *[KeySize]byte) {
	key[0] &= 248
	key[31] &= 127
	key[31] |= 64
}

// Ed25519ToX25519Private converts an Ed25519 private key to X25519.
func Ed25519ToX25519Private(edPriv ed25519.PrivateKey) []byte {
	h := sha512.Sum512(edPriv.Seed())
	h[0] &= 248
	h[31] &= 127
	h[31] |= 64
	return h[:KeySize]
}

// Ed25519ToX25519Public converts an Ed25519 public key to X25519
// using proper Edwards-to-Montgomery conversion.
func Ed25519ToX25519Public(edPub ed25519.PublicKey) ([]byte, error) {
	p, err := new(edwards25519.Point).SetBytes(edPub)
	if err != nil {
		return nil, fmt.Errorf("invalid ed25519 public key: %w", err)
	}
	return p.BytesMontgomery(), nil
}

// Sign signs a message with an Ed25519 private key.
func Sign(privateKey, message []byte) []byte {
	return ed25519.Sign(ed25519.PrivateKey(privateKey), message)
}

// Verify checks an Ed25519 signature.
func Verify(publicKey, message, signature []byte) bool {
	if len(publicKey) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(ed25519.PublicKey(publicKey), message, signature)
}

// X3DH performs a simplified X3DH key agreement.
// Returns a shared secret derived from the DH exchanges.
func X3DH(ourX25519Priv []byte, theirSignedPreKey []byte, theirOneTimeKey []byte) ([]byte, error) {
	// DH1: our identity x25519 * their signed pre-key
	dh1, err := curve25519.X25519(ourX25519Priv, theirSignedPreKey)
	if err != nil {
		return nil, fmt.Errorf("dh1: %w", err)
	}

	// Generate ephemeral key for DH2
	ephemeral, err := GenerateX25519()
	if err != nil {
		return nil, fmt.Errorf("ephemeral: %w", err)
	}

	// DH2: ephemeral * their signed pre-key
	dh2, err := curve25519.X25519(ephemeral.Private, theirSignedPreKey)
	if err != nil {
		return nil, fmt.Errorf("dh2: %w", err)
	}

	// Combine DH results
	input := append(dh1, dh2...)

	// DH3: ephemeral * their one-time key (if available)
	if len(theirOneTimeKey) == KeySize {
		dh3, err := curve25519.X25519(ephemeral.Private, theirOneTimeKey)
		if err == nil {
			input = append(input, dh3...)
		}
	}

	// Derive shared secret using HKDF
	return deriveKey(input, []byte("cgram-x3dh-v1"))
}

// DeriveSharedSecret creates a shared secret from a DH exchange for an existing session.
func DeriveSharedSecret(ourPriv, theirPub []byte) ([]byte, error) {
	shared, err := curve25519.X25519(ourPriv, theirPub)
	if err != nil {
		return nil, err
	}
	return deriveKey(shared, []byte("cgram-session-v1"))
}

func deriveKey(input, info []byte) ([]byte, error) {
	r := hkdf.New(sha256.New, input, hkdfSalt, info)
	key := make([]byte, KeySize)
	if _, err := io.ReadFull(r, key); err != nil {
		return nil, fmt.Errorf("hkdf: %w", err)
	}
	return key, nil
}

// Encrypt encrypts plaintext using the shared secret and ratchet index.
func Encrypt(sharedSecret []byte, ratchetIndex uint32, plaintext []byte) ([]byte, error) {
	// Derive message key from shared secret + ratchet index
	msgKey, err := deriveMessageKey(sharedSecret, ratchetIndex)
	if err != nil {
		return nil, err
	}

	var key [KeySize]byte
	copy(key[:], msgKey)

	var nonce [NonceSize]byte
	binary.BigEndian.PutUint32(nonce[:4], ratchetIndex)
	if _, err := rand.Read(nonce[8:]); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	encrypted := secretbox.Seal(nonce[:], plaintext, &nonce, &key)
	return encrypted, nil
}

// Decrypt decrypts ciphertext using the shared secret and ratchet index.
func Decrypt(sharedSecret []byte, ratchetIndex uint32, ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < NonceSize+secretbox.Overhead {
		return nil, fmt.Errorf("ciphertext too short")
	}

	msgKey, err := deriveMessageKey(sharedSecret, ratchetIndex)
	if err != nil {
		return nil, err
	}

	var key [KeySize]byte
	copy(key[:], msgKey)

	var nonce [NonceSize]byte
	copy(nonce[:], ciphertext[:NonceSize])

	plaintext, ok := secretbox.Open(nil, ciphertext[NonceSize:], &nonce, &key)
	if !ok {
		return nil, fmt.Errorf("decryption failed")
	}

	return plaintext, nil
}

func deriveMessageKey(sharedSecret []byte, index uint32) ([]byte, error) {
	indexBytes := make([]byte, 4)
	binary.BigEndian.PutUint32(indexBytes, index)

	input := make([]byte, len(sharedSecret)+4)
	copy(input, sharedSecret)
	copy(input[len(sharedSecret):], indexBytes)
	return deriveKey(input, []byte("cgram-msg-v1"))
}

// GeneratePreKeyBundle creates a set of pre-keys for X3DH.
func GeneratePreKeyBundle(identityPrivate []byte, n int) (signedPreKey *KeyPair, signature []byte, oneTimeKeys []*KeyPair, err error) {
	signedPreKey, err = GenerateX25519()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("signed pre-key: %w", err)
	}

	signature = Sign(identityPrivate, signedPreKey.Public)

	oneTimeKeys = make([]*KeyPair, n)
	for i := range n {
		oneTimeKeys[i], err = GenerateX25519()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("one-time key %d: %w", i, err)
		}
	}

	return signedPreKey, signature, oneTimeKeys, nil
}
