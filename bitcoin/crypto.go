package bitcoin

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"math/big"

	"github.com/pkg/errors"
)

// ECDHSecret returns the secret derived using ECDH (Elliptic Curve Diffie Hellman).
func ECDHSecret(k Key, pub PublicKey) ([]byte, error) {
	var x, y big.Int
	pubX, pubY := pub.Numbers()
	x.SetBytes(pubX)
	y.SetBytes(pubY)

	sx, _ := curveS256.ScalarMult(&x, &y, k.Number()) // DH is just k * pub
	return sx.Bytes(), nil
}

// Encrypt generates a random IV prepends it to the output, then uses AES with the input keysize and
//   CBC to encrypt the payload.
func Encrypt(payload, key []byte) ([]byte, error) {
	// Generate random IV
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, errors.Wrap(err, "rand iv")
	}

	return EncryptIV(payload, key, iv)
}

// EncryptIV uses AES with the input keysize and the specified IV and CBC to encrypt the payload.
func EncryptIV(payload, key, iv []byte) ([]byte, error) {
	// Append 0xff to end of payload so padding, for block alignment, can be removed.
	size := len(payload)
	newSize := size + 1
	if newSize%aes.BlockSize != 0 {
		newSize = newSize + (aes.BlockSize - (newSize % aes.BlockSize))
	}
	plaintext := make([]byte, newSize)
	copy(plaintext, payload)
	plaintext[size] = 0xff

	aesCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "new cipher")
	}

	ciphertext := make([]byte, aes.BlockSize+len(plaintext))
	copy(ciphertext, iv)

	mode := cipher.NewCBCEncrypter(aesCipher, iv)
	mode.CryptBlocks(ciphertext[aes.BlockSize:], plaintext)

	return ciphertext, nil
}

// Decrypt reads the IV from the beginning of the output, then uses AES with the input keysize and
//   CBC to decrypt the payload.
func Decrypt(payload, key []byte) ([]byte, error) {
	size := len(payload)
	if size == 0 {
		return nil, nil
	}
	if size <= aes.BlockSize {
		return nil, errors.New("Payload too short for decrypt")
	}

	if len(payload)%aes.BlockSize != 0 {
		return nil, errors.New("Payload size doesn't align with decrypt block size")
	}

	aesCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "new cipher")
	}

	iv := payload[:aes.BlockSize]
	ciphertext := payload[aes.BlockSize:]
	plaintext := make([]byte, len(ciphertext))

	mode := cipher.NewCBCDecrypter(aesCipher, iv)
	mode.CryptBlocks(plaintext, ciphertext)

	// Trim padding by looking for appended 0xff.
	found := false
	stop := 0
	if len(plaintext) > aes.BlockSize {
		stop = len(plaintext) - aes.BlockSize
	}
	payloadLength := 0
	for i := len(plaintext) - 1; ; i-- {
		if plaintext[i] == 0xff {
			found = true
			payloadLength = i
			break
		}
		if i == stop {
			break
		}
	}

	if !found {
		return nil, errors.New("Missing padding marker")
	}

	return plaintext[:payloadLength], nil
}
