// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2_test

import (
	"bytes"
	"crypto/aes"
	"crypto/rand"
	"testing"

	. "github.com/chrisccoulson/go-tpm2"
)

func TestSymmetricAES(t *testing.T) {
	for _, data := range []struct {
		desc      string
		keyLength int
		data      []byte
	}{
		{
			desc:      "128",
			keyLength: 16,
			data:      []byte("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"),
		},
		{
			desc:      "256",
			keyLength: 32,
			data:      []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"),
		},
	} {
		t.Run(data.desc, func(t *testing.T) {
			key := make([]byte, data.keyLength)
			rand.Read(key)

			iv := make([]byte, aes.BlockSize)
			rand.Read(iv)

			var secret []byte
			secret = append(secret, data.data...)

			if err := TestCryptEncryptSymmetricAES(key, SymModeCFB, secret, iv); err != nil {
				t.Fatalf("AES encryption failed: %v", err)
			}

			if err := TestCryptDecryptSymmetricAES(key, SymModeCFB, secret, iv); err != nil {
				t.Fatalf("AES decryption failed: %v", err)
			}

			if !bytes.Equal(secret, data.data) {
				t.Errorf("Encrypt / decrypt with AES didn't produce the original data")
			}
		})
	}
}

func TestXORObfuscation(t *testing.T) {
	for _, data := range []struct {
		desc      string
		keyLength int
		alg       HashAlgorithmId
		data      []byte
	}{
		{
			desc:      "SHA256/1",
			keyLength: 32,
			alg:       HashAlgorithmSHA256,
			data:      []byte("secret data"),
		},
		{
			desc:      "SHA256/2",
			keyLength: 60,
			alg:       HashAlgorithmSHA256,
			data:      []byte("super secret data"),
		},
		{
			desc:      "SHA1/1",
			keyLength: 60,
			alg:       HashAlgorithmSHA1,
			data:      []byte("XXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXXX"),
		},
	} {
		t.Run(data.desc, func(t *testing.T) {
			key := make([]byte, data.keyLength)
			rand.Read(key)

			digestSize := data.alg.Size()

			contextU := make([]byte, digestSize)
			rand.Read(contextU)

			contextV := make([]byte, digestSize)
			rand.Read(contextV)

			var secret []byte
			secret = append(secret, data.data...)

			if err := TestCryptXORObfuscation(data.alg, key, contextU, contextV, secret); err != nil {
				t.Fatalf("XOR obfuscation failed: %v", err)
			}

			if err := TestCryptXORObfuscation(data.alg, key, contextU, contextV, secret); err != nil {
				t.Fatalf("XOR obfuscation failed: %v", err)
			}

			if !bytes.Equal(secret, data.data) {
				t.Errorf("Encrypt / decrypt with XOR obfuscation didn't produce the original data")
			}
		})
	}
}
