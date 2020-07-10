package bitcoin

import (
	"bytes"
	"crypto/aes"
	"encoding/hex"
	"fmt"
	"testing"
)

func TestEncryptor(t *testing.T) {
	tests := []struct {
		key       string
		iv        string
		data      string
		encrypted string
	}{
		{
			key:       "0000000000000000000000000000000000000000000000000000000000000000",
			iv:        "00000000000000000000000000000000",
			data:      "80000000000000000000000000000000",
			encrypted: "ddc6bf790c15760d8d9aeb6f9a75fd4e",
		},
		{
			key:       "0000000000000000000000000000000000000000000000000000000000000000",
			iv:        "00000000000000000000000000000000",
			data:      "ffffffffffff00000000000000000000",
			encrypted: "ead731af4d3a2fe3b34bed047942a49f",
		},
		{
			key:       "0000000000000000000000000000000000000000000000000000000000000000",
			iv:        "00000000000000000000000000000000",
			data:      "ffffe000000000000000000000000000",
			encrypted: "2239455e7afe3b0616100288cc5a723b",
		},
		{
			key:       "0000000000000000000000000000000000000000000000000000000000000000",
			iv:        "00000000000000000000000000000000",
			data:      "fffffffffffffffffffffffffffffffe",
			encrypted: "7bfe9d876c6d63c1d035da8fe21c409d",
		},
		{
			key:       "ffffffffffffffffff0000000000000000000000000000000000000000000000",
			iv:        "00000000000000000000000000000000",
			data:      "00000000000000000000000000000000",
			encrypted: "4b3b9f1e099c2a09dc091e90e4f18f0a",
		},
		{
			key:       "fffffffffffffffffffff8000000000000000000000000000000000000000000",
			iv:        "00000000000000000000000000000000",
			data:      "00000000000000000000000000000000",
			encrypted: "ab0c8410aeeead92feec1eb430d652cb",
		},
		{
			key:       "0000000000000000000000000000000000000000000000000000000000000000",
			iv:        "00000000000000000000000000000000",
			data:      "014730f80ac625fe84f026c60bfd547d",
			encrypted: "5c9d844ed46f9885085e5d6a4f94c7d7",
		},
		{
			key:       "0000000000000000000000000000000000000000000000000000000000000000",
			iv:        "00000000000000000000000000000000",
			data:      "91fbef2d15a97816060bee1feaa49afe",
			encrypted: "1bc704f1bce135ceb810341b216d7abe",
		},
		{
			key:       "c47b0294dbbbee0fec4757f22ffeee3587ca4730c3d33b691df38bab076bc558",
			iv:        "00000000000000000000000000000000",
			data:      "00000000000000000000000000000000",
			encrypted: "46f2fb342d6f0ab477476fc501242c5f",
		},
		{
			key:       "f8be9ba615c5a952cabbca24f68f8593039624d524c816acda2c9183bd917cb9",
			iv:        "00000000000000000000000000000000",
			data:      "00000000000000000000000000000000",
			encrypted: "a3944b95ca0b52043584ef02151926a8",
		},
		{
			key:       "fe8901fecd3ccd2ec5fdc7c7a0b50519c245b42d611a5ef9e90268d59f3edf33",
			iv:        "bd416cb3b9892228d8f1df575692e4d0",
			data:      "8d3aa196ec3d7c9b5bb122e7fe77fb1295a6da75abe5d3a510194d3a8a4157d5c89d40619716619859da3ec9b247ced9",
			encrypted: "608e82c7ab04007adb22e389a44797fed7de090c8c03ca8a2c5acd9e84df37fbc58ce8edb293e98f02b640d6d1d72464",
		},
		{
			key:       "9adc8fbd506e032af7fa20cf5343719de6d1288c158c63d6878aaf64ce26ca85",
			iv:        "11958dc6ab81e1c7f01631e9944e620f",
			data:      "c7917f84f747cd8c4b4fedc2219bdbc5f4d07588389d8248854cf2c2f89667a2d7bcf53e73d32684535f42318e24cd45793950b3825e5d5c5c8fcd3e5dda4ce9246d18337ef3052d8b21c5561c8b660e",
			encrypted: "9c99e68236bb2e929db1089c7750f1b356d39ab9d0c40c3e2f05108ae9d0c30b04832ccdbdc08ebfa426b7f5efde986ed05784ce368193bb3699bc691065ac62e258b9aa4cc557e2b45b49ce05511e65",
		},
		{
			key:       "48be597e632c16772324c8d3fa1d9c5a9ecd010f14ec5d110d3bfec376c5532b",
			iv:        "d6d581b8cf04ebd3b6eaa1b53f047ee1",
			data:      "0c63d413d3864570e70bb6618bf8a4b9585586688c32bba0a5ecc1362fada74ada32c52acfd1aa7444ba567b4e7daaecf7cc1cb29182af164ae5232b002868695635599807a9a7f07a1f137e97b1e1c9dabc89b6a5e4afa9db5855edaa575056a8f4f8242216242bb0c256310d9d329826ac353d715fa39f80cec144d6424558f9f70b98c920096e0f2c855d594885a00625880e9dfb734163cecef72cf030b8",
			encrypted: "fc5873e50de8faf4c6b84ba707b0854e9db9ab2e9f7d707fbba338c6843a18fc6facebaf663d26296fb329b4d26f18494c79e09e779647f9bafa87489630d79f4301610c2300c19dbf3148b7cac8c4f4944102754f332e92b6f7c5e75bc6179eb877a078d4719009021744c14f13fd2a55a2b9c44d18000685a845a4f632c7c56a77306efa66a24d05d088dcd7c13fe24fc447275965db9e4d37fbc9304448cd",
		},
		{
			key:       "48be597e632c16772324c8d3fa1d9c5a9ecd010f14ec5d110d3bfec376c5532b",
			iv:        "d6d581b8cf04ebd3b6eaa1b53f047ee1",
			data:      "0c63d413d3864570e70bb6618bf8a4b9585586688c32bba0a5ecc1362fada74ada32c52acfd1aa7444ba567b4e7daaecf7cc1cb29182af164ae5232b002868695635599807a9a7f07a1f137e97b1e1c9dabc89b6a5e4afa9db5855edaa575056a8f4f8242216242bb0c256310d9d329826ac353d715fa39f80cec144d6424558f9f70b98c920096e0f2c855d594885a00625880e9dfb734163cece",
			encrypted: "fc5873e50de8faf4c6b84ba707b0854e9db9ab2e9f7d707fbba338c6843a18fc6facebaf663d26296fb329b4d26f18494c79e09e779647f9bafa87489630d79f4301610c2300c19dbf3148b7cac8c4f4944102754f332e92b6f7c5e75bc6179eb877a078d4719009021744c14f13fd2a55a2b9c44d18000685a845a4f632c7c56a77306efa66a24d05d088dcd7c13fe23b902c23416f6094e3d580",
		},
	}

	for i, tt := range tests {
		t.Run(fmt.Sprintf("Test-%d", i+1), func(t *testing.T) {
			key, err := hex.DecodeString(tt.key)
			if err != nil {
				t.Fatalf("Failed to decode hex key : %s", err)
			}

			iv, err := hex.DecodeString(tt.iv)
			if err != nil {
				t.Fatalf("Failed to decode hex iv : %s", err)
			}

			data, err := hex.DecodeString(tt.data)
			if err != nil {
				t.Fatalf("Failed to decode hex data : %s", err)
			}

			encrypted, err := hex.DecodeString(tt.encrypted)
			if err != nil {
				t.Fatalf("Failed to decode hex encrypted : %s", err)
			}

			// Test Encrypt/Decrypt ----------------------------------------------------------------
			resultEncrypted, err := EncryptIV(data, key, iv)
			if err != nil {
				t.Fatalf("Failed to encrypt data : %s", err)
			}

			if !bytes.Equal(resultEncrypted[:aes.BlockSize], iv) {
				t.Errorf("Wrong iv value : \n  got  %x\n  want %x",
					resultEncrypted[:aes.BlockSize], iv)
			}

			if len(resultEncrypted) < aes.BlockSize+len(data) {
				t.Fatalf("Raw encrypted too short : got %d, want > %d", len(resultEncrypted),
					(aes.BlockSize*2)+len(data))
			}
			if !bytes.Equal(resultEncrypted[aes.BlockSize:aes.BlockSize+len(data)], encrypted) {
				t.Errorf("Wrong encrypted value : \n  got  %x\n  want %x",
					resultEncrypted[aes.BlockSize:aes.BlockSize+len(data)], encrypted)
			}

			resultDecrypted, err := Decrypt(resultEncrypted, key)
			if err != nil {
				t.Fatalf("Failed to decrypt data : %s", err)
			}
			if !bytes.Equal(resultDecrypted, data) {
				t.Errorf("Wrong decrypted value : \n  got  %x\n  want %x", resultDecrypted, data)
			}

			// Test Encryptor/Decryptor (Stream encryption) ----------------------------------------
			var buf bytes.Buffer
			e, err := NewEncryptorIV(key, iv, &buf)
			if err != nil {
				t.Fatalf("Failed create encryptor : %s", err)
			}

			if err := e.Write(data); err != nil {
				t.Fatalf("Failed to encrypt data : %s", err)
			}

			if err := e.Close(); err != nil {
				t.Fatalf("Failed to close encryptor : %s", err)
			}

			rawEncrypted := buf.Bytes()
			if !bytes.Equal(rawEncrypted[:aes.BlockSize], iv) {
				t.Errorf("Wrong iv value : \n  got  %x\n  want %x",
					rawEncrypted[:aes.BlockSize], iv)
			}

			if len(rawEncrypted) < aes.BlockSize+len(data) {
				t.Fatalf("Raw encrypted too short : got %d, want > %d", len(rawEncrypted),
					(aes.BlockSize*2)+len(data))
			}
			rawEncrypted = rawEncrypted[aes.BlockSize : aes.BlockSize+len(data)]
			if !bytes.Equal(rawEncrypted, encrypted) {
				t.Errorf("Wrong encrypted value : \n  got  %x\n  want %x", rawEncrypted, encrypted)
			}

			d, err := NewDecryptor(key, bytes.NewReader(buf.Bytes()))
			if err != nil {
				t.Fatalf("Failed to create decryptor : %s", err)
			}

			resultData := make([]byte, len(data))
			if err := d.Read(resultData); err != nil {
				t.Fatalf("Failed to decrypt data : %s", err)
			}

			if !bytes.Equal(resultData, data) {
				t.Errorf("Wrong decrypted value : \n  got  %x\n  want %x", resultData, data)
			}

			complete, err := d.IsComplete()
			if err != nil {
				t.Fatalf("Failed to check decryptor is complete : %s", err)
			}
			if !complete {
				t.Errorf("Decryptor is not complete")
			}
		})
	}
}
