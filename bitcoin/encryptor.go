package bitcoin

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/pkg/errors"
)

// Encryptor is an io.WriteCloser (Writer and Closer) the encrypts data with a specified key.
type Encryptor struct {
	cipher.BlockMode
	w         io.Writer
	remainder []byte
}

// Decryptor is an io.Reader that decrypts data with a specified key
type Decryptor struct {
	cipher.BlockMode
	r         io.Reader
	remainder []byte
}

// NewEncryptor creates a new encryptor.
// w is the writer that the encrypted data should be written to.
func NewEncryptor(key []byte, w io.Writer) (*Encryptor, error) {
	// Create random initialization vector (IV)
	iv := make([]byte, aes.BlockSize)
	if _, err := rand.Read(iv); err != nil {
		return nil, errors.Wrap(err, "random read")
	}

	return NewEncryptorIV(key, iv, w)
}

// NewEncryptorIV creates a new encryptor with a specified initialization vector.
// w is the writer that the encrypted data should be written to.
func NewEncryptorIV(key, iv []byte, w io.Writer) (*Encryptor, error) {
	if len(iv) != aes.BlockSize {
		return nil, fmt.Errorf("Invalid IV size : %d should be %d", len(iv), aes.BlockSize)
	}

	result := &Encryptor{w: w}

	// Create cipher
	aesCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "new cipher")
	}

	// Write initialization vector (IV)
	if _, err := w.Write(iv); err != nil {
		return nil, errors.Wrap(err, "write iv")
	}

	// Create block mode encryptor
	result.BlockMode = cipher.NewCBCEncrypter(aesCipher, iv)

	return result, nil
}

// Write implements io.Writer
func (e *Encryptor) Write(b []byte) error {
	lr := len(e.remainder)
	lb := len(b)
	remaining := lr + lb

	if remaining < aes.BlockSize {
		// not enough to fill a block
		e.remainder = append(e.remainder, b...)
		return nil
	}

	block := make([]byte, aes.BlockSize)
	encrypted := make([]byte, aes.BlockSize)

	// Append new data to previous remainder to complete first block
	copy(block, e.remainder)
	offset := copy(block[lr:], b)
	remaining -= aes.BlockSize

	// Encrypt as many full blocks as are available
	for {
		// Encrypt block
		e.BlockMode.CryptBlocks(encrypted, block)

		// Write encrypted block
		if _, err := e.w.Write(encrypted); err != nil {
			return errors.Wrap(err, "write")
		}

		if remaining < aes.BlockSize {
			if remaining == 0 {
				e.remainder = nil
				return nil
			}

			// Create separate slice for remaining data to allow freeing of input slice.
			e.remainder = make([]byte, remaining)
			copy(e.remainder, b[offset:])
			return nil
		}

		// Prepare next block
		copy(block, b[offset:])
		offset += aes.BlockSize
		remaining -= aes.BlockSize
	}

	return nil
}

// Close implements io.Closer
func (e *Encryptor) Close() error {
	// Append 0xff to end of payload so padding, for block alignment, can be removed during
	// decryption.
	block := make([]byte, aes.BlockSize)

	lr := len(e.remainder)
	copy(block, e.remainder)
	copy(block[lr:], []byte{0xff})

	// Encrypt block
	encrypted := make([]byte, aes.BlockSize)
	e.BlockMode.CryptBlocks(encrypted, block)

	// Write encrypted block
	if _, err := e.w.Write(encrypted); err != nil {
		return errors.Wrap(err, "write")
	}

	e.remainder = nil
	return nil
}

// NewDecryptor creates a new decryptor.
// r is the reader that the encrypted data should be read from.
func NewDecryptor(key []byte, r io.Reader) (*Decryptor, error) {
	result := &Decryptor{r: r}

	// Create cipher
	aesCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, errors.Wrap(err, "new cipher")
	}

	// Read initialization vector (IV)
	iv := make([]byte, aes.BlockSize)
	if _, err = io.ReadFull(r, iv); err != nil {
		return nil, errors.Wrap(err, "read iv")
	}

	// Create block mode encryptor
	result.BlockMode = cipher.NewCBCDecrypter(aesCipher, iv)

	return result, nil
}

// Read implements io.Reader
func (e *Decryptor) Read(b []byte) error {
	lr := len(e.remainder)
	offset := copy(b, e.remainder)

	if offset < lr {
		// Read was less than remainder
		e.remainder = e.remainder[offset:]
		return nil
	}

	// Remainder used, so clear it.
	e.remainder = nil

	lb := len(b)
	if offset == lb {
		// Result is full
		return nil
	}

	for {
		// Read another block
		encrypted := make([]byte, aes.BlockSize)
		if _, err := e.r.Read(encrypted); err != nil {
			return errors.Wrap(err, "read block")
		}

		block := make([]byte, aes.BlockSize)
		e.BlockMode.CryptBlocks(block, encrypted)
		used := copy(b[offset:], block)

		if used < aes.BlockSize {
			// Result is full. Save remainder and return.
			e.remainder = block[used:]
			return nil
		}

		offset += used
		if offset == lb {
			// Result is full. There is no remainder
			e.remainder = nil
			return nil
		}
	}

	return nil
}

// IsComplete returns true if the end of the encrypted stream has been reached
func (e *Decryptor) IsComplete() (bool, error) {
	if len(e.remainder) == 0 {
		// Read another block
		encrypted := make([]byte, aes.BlockSize)
		if _, err := e.r.Read(encrypted); err != nil {
			return false, errors.Wrap(err, "read block")
		}

		e.remainder = make([]byte, aes.BlockSize)
		e.BlockMode.CryptBlocks(e.remainder, encrypted)
	}

	return e.remainder[0] == 0xff, nil
}
