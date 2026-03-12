package crypto

import (
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"os"

	"filippo.io/edwards25519"
	"golang.org/x/crypto/curve25519"
	"golang.org/x/crypto/nacl/box"

	pb "github.com/isalikov/cgram-proto/gen/proto"
)

// KeyPair holds an Ed25519 identity key pair.
type KeyPair struct {
	PublicKey  ed25519.PublicKey  `json:"public_key"`
	PrivateKey ed25519.PrivateKey `json:"private_key"`
}

// Session holds the shared secret for a peer.
type Session struct {
	PeerID       string   `json:"peer_id"`
	SharedSecret [32]byte `json:"shared_secret"`
}

// PreKeyResult holds the bundle to upload and the private keys to store locally.
type PreKeyResult struct {
	Bundle          *pb.PreKeyBundle
	SignedPreKeyPriv [32]byte
	OneTimeKeyPrivs  [][32]byte
}

// LoadOrCreateIdentity loads or generates an Ed25519 key pair.
func LoadOrCreateIdentity(path string) (*KeyPair, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		// Verify file permissions (should be 0600)
		info, statErr := os.Stat(path)
		if statErr == nil && info.Mode().Perm()&0077 != 0 {
			if chErr := os.Chmod(path, 0600); chErr != nil {
				return nil, fmt.Errorf("key file has insecure permissions and chmod failed: %w", chErr)
			}
		}

		var kp KeyPair
		if err := json.Unmarshal(data, &kp); err != nil {
			return nil, fmt.Errorf("parse key file: %w", err)
		}
		return &kp, nil
	}

	pub, priv, err := ed25519.GenerateKey(cryptorand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate key: %w", err)
	}

	kp := &KeyPair{PublicKey: pub, PrivateKey: priv}
	data, _ = json.Marshal(kp)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return nil, fmt.Errorf("save key: %w", err)
	}
	return kp, nil
}

// GeneratePreKeyBundle creates signed pre-key and one-time keys for upload.
// Returns both the bundle (for server) and private keys (for local storage).
func GeneratePreKeyBundle(identity *KeyPair) (*PreKeyResult, error) {
	// Generate X25519 signed pre-key
	signedPreKeyPriv, signedPreKeyPub, err := generateX25519KeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate signed pre-key: %w", err)
	}

	// Sign the pre-key with identity key
	signature := ed25519.Sign(identity.PrivateKey, signedPreKeyPub[:])

	// Generate one-time pre-keys
	var oneTimeKeys [][]byte
	var oneTimePrivs [][32]byte
	for i := 0; i < 20; i++ {
		priv, pub, err := generateX25519KeyPair()
		if err != nil {
			return nil, fmt.Errorf("generate one-time key %d: %w", i, err)
		}
		oneTimeKeys = append(oneTimeKeys, pub[:])
		oneTimePrivs = append(oneTimePrivs, priv)
	}

	return &PreKeyResult{
		Bundle: &pb.PreKeyBundle{
			IdentityKey:           identity.PublicKey,
			SignedPreKey:          signedPreKeyPub[:],
			SignedPreKeySignature: signature,
			OneTimePreKeys:        oneTimeKeys,
		},
		SignedPreKeyPriv: signedPreKeyPriv,
		OneTimeKeyPrivs:  oneTimePrivs,
	}, nil
}

// Ed25519ToX25519 converts an Ed25519 public key to X25519 using proper
// Edwards-to-Montgomery point conversion.
func Ed25519ToX25519(edPub ed25519.PublicKey) ([32]byte, error) {
	var x25519Pub [32]byte
	if len(edPub) != 32 {
		return x25519Pub, fmt.Errorf("invalid ed25519 public key length")
	}

	p, err := new(edwards25519.Point).SetBytes(edPub)
	if err != nil {
		return x25519Pub, fmt.Errorf("invalid ed25519 public key: %w", err)
	}

	copy(x25519Pub[:], p.BytesMontgomery())
	return x25519Pub, nil
}

func generateX25519KeyPair() (priv [32]byte, pub [32]byte, err error) {
	if _, err = cryptorand.Read(priv[:]); err != nil {
		return
	}
	curve25519.ScalarBaseMult(&pub, &priv)
	return
}

// Encrypt encrypts a message using NaCl box with the shared secret.
func Encrypt(message []byte, sharedSecret *[32]byte) ([]byte, error) {
	var nonce [24]byte
	if _, err := cryptorand.Read(nonce[:]); err != nil {
		return nil, fmt.Errorf("generate nonce: %w", err)
	}

	encrypted := box.SealAfterPrecomputation(nonce[:], message, &nonce, sharedSecret)
	return encrypted, nil
}

// Decrypt decrypts a message using NaCl box with the shared secret.
func Decrypt(encrypted []byte, sharedSecret *[32]byte) ([]byte, error) {
	if len(encrypted) < 24 {
		return nil, fmt.Errorf("ciphertext too short")
	}

	var nonce [24]byte
	copy(nonce[:], encrypted[:24])

	decrypted, ok := box.OpenAfterPrecomputation(nil, encrypted[24:], &nonce, sharedSecret)
	if !ok {
		return nil, fmt.Errorf("decryption failed")
	}

	return decrypted, nil
}
