// Copyright 2021 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package util

import (
	"bytes"
	"crypto/hmac"
	"crypto/rand"
	"errors"
	"hash"
	"io/ioutil"

	"golang.org/x/xerrors"

	"github.com/canonical/go-tpm2"
	"github.com/canonical/go-tpm2/internal"
	"github.com/canonical/go-tpm2/mu"
)

// UnwrapOuter removes an outer wrapper from the supplied sensitive data blob. The
// supplied name is associated with the data.
//
// It checks the integrity HMAC is valid using the specified digest algorithm and
// a key derived from the supplied seed and returns an error if the check fails.
//
// It then decrypts the data blob using the specified symmetric algorithm and a
// key derived from the supplied seed and name.
func UnwrapOuter(hashAlg tpm2.HashAlgorithmId, symmetricAlg *tpm2.SymDefObject, name tpm2.Name, seed []byte, useIV bool, data []byte) ([]byte, error) {
	r := bytes.NewReader(data)

	var integrity []byte
	if _, err := mu.UnmarshalFromReader(r, &integrity); err != nil {
		return nil, xerrors.Errorf("cannot unpack integrity digest: %w", err)
	}

	data, _ = ioutil.ReadAll(r)

	hmacKey := internal.KDFa(hashAlg.GetHash(), seed, []byte(tpm2.IntegrityKey), nil, nil, hashAlg.Size()*8)
	h := hmac.New(func() hash.Hash { return hashAlg.NewHash() }, hmacKey)
	h.Write(data)
	h.Write(name)

	if !bytes.Equal(h.Sum(nil), integrity) {
		return nil, errors.New("integrity digest is invalid")
	}

	r = bytes.NewReader(data)

	iv := make([]byte, symmetricAlg.Algorithm.BlockSize())
	if useIV {
		if _, err := mu.UnmarshalFromReader(r, &iv); err != nil {
			return nil, xerrors.Errorf("cannot unpack IV: %w", err)
		}
		if len(iv) != symmetricAlg.Algorithm.BlockSize() {
			return nil, errors.New("IV has the wrong size")
		}
	}

	data, _ = ioutil.ReadAll(r)

	symKey := internal.KDFa(hashAlg.GetHash(), seed, []byte(tpm2.StorageKey), name, nil, int(symmetricAlg.KeyBits.Sym))

	if err := tpm2.CryptSymmetricDecrypt(tpm2.SymAlgorithmId(symmetricAlg.Algorithm), symKey, iv, data); err != nil {
		return nil, xerrors.Errorf("cannot remove wrapper: %w", err)
	}

	return data, nil
}

// ProduceOuterWrap adds an outer wrapper to the supplied data. The supplied name
// is associated with the data.
//
// It encrypts the data using the specified symmetric algorithm and a key derived
// from the supplied seed and name.
//
// It then prepends an integrity HMAC of the encrypted data and the supplied
// name using the specified digest algorithm and a key derived from the supplied
// seed.
func ProduceOuterWrap(hashAlg tpm2.HashAlgorithmId, symmetricAlg *tpm2.SymDefObject, name tpm2.Name, seed []byte, useIV bool, data []byte) ([]byte, error) {
	iv := make([]byte, symmetricAlg.Algorithm.BlockSize())
	if useIV {
		if _, err := rand.Read(iv); err != nil {
			return nil, xerrors.Errorf("cannot generate IV: %w", err)
		}
	}

	symKey := internal.KDFa(hashAlg.GetHash(), seed, []byte(tpm2.StorageKey), name, nil, int(symmetricAlg.KeyBits.Sym))

	if err := tpm2.CryptSymmetricEncrypt(tpm2.SymAlgorithmId(symmetricAlg.Algorithm), symKey, iv, data); err != nil {
		return nil, xerrors.Errorf("cannot apply wrapper: %w", err)
	}

	if useIV {
		data = mu.MustMarshalToBytes(iv, mu.RawBytes(data))
	}

	hmacKey := internal.KDFa(hashAlg.GetHash(), seed, []byte(tpm2.IntegrityKey), nil, nil, hashAlg.Size()*8)
	h := hmac.New(func() hash.Hash { return hashAlg.NewHash() }, hmacKey)
	h.Write(data)
	h.Write(name)

	integrity := h.Sum(nil)

	return mu.MustMarshalToBytes(integrity, mu.RawBytes(data)), nil
}

func PrivateToSensitive(private tpm2.Private, name tpm2.Name, hashAlg tpm2.HashAlgorithmId, symmetricAlg *tpm2.SymDefObject, seed []byte) (*tpm2.Sensitive, error) {
	data, err := UnwrapOuter(hashAlg, symmetricAlg, name, seed, true, private)
	if err != nil {
		return nil, xerrors.Errorf("cannot unwrap outer wrapper: %w", err)
	}

	var sensitive struct {
		Ptr *tpm2.Sensitive `tpm2:"sized"`
	}
	if _, err := mu.UnmarshalFromBytes(data, &sensitive); err != nil {
		return nil, xerrors.Errorf("cannot unmarhsal sensitive: %w", err)
	}

	return sensitive.Ptr, nil
}

func SensitiveToPrivate(sensitive *tpm2.Sensitive, name tpm2.Name, hashAlg tpm2.HashAlgorithmId, symmetricAlg *tpm2.SymDefObject, seed []byte) (tpm2.Private, error) {
	sensitiveSized := struct {
		Ptr *tpm2.Sensitive `tpm2:"sized"`
	}{sensitive}
	private, err := mu.MarshalToBytes(sensitiveSized)
	if err != nil {
		return nil, xerrors.Errorf("cannot marshal sensitive: %w", err)
	}

	private, err = ProduceOuterWrap(hashAlg, symmetricAlg, name, seed, true, private)
	if err != nil {
		return nil, xerrors.Errorf("cannot apply outer wrapper: %w", err)
	}

	return private, nil
}

// DuplicateToSensitive unwraps the supplied duplication blob. The supplied name
// is the name of the duplication object.
//
// If a seed is supplied, it removes the outer wrapper using the specified parent
// name algorithm and parent symmetric algorithm - these correspond to properties of
// the new parent's public area.
//
// If symmetricAlg is supplied, it removes the inner wrapper - first by decrypting
// it with the supplied innerSymKey, and then checking the inner integrity digest
// is valid and returning an error if it isn't.
func DuplicateToSensitive(duplicate tpm2.Private, name tpm2.Name, parentNameAlg tpm2.HashAlgorithmId, parentSymmetricAlg *tpm2.SymDefObject, seed []byte, symmetricAlg *tpm2.SymDefObject, innerSymKey tpm2.Data) (*tpm2.Sensitive, error) {
	if len(seed) > 0 {
		// Remove outer wrapper
		var err error
		duplicate, err = UnwrapOuter(parentNameAlg, parentSymmetricAlg, name, seed, false, duplicate)
		if err != nil {
			return nil, xerrors.Errorf("cannot unwrap outer wrapper: %w", err)
		}
	}

	if symmetricAlg != nil && symmetricAlg.Algorithm != tpm2.SymObjectAlgorithmNull {
		// Remove inner wrapper
		if err := tpm2.CryptSymmetricDecrypt(tpm2.SymAlgorithmId(symmetricAlg.Algorithm), innerSymKey, make([]byte, symmetricAlg.Algorithm.BlockSize()), duplicate); err != nil {
			return nil, xerrors.Errorf("cannot remove inner wrapper: %w", err)
		}

		r := bytes.NewReader(duplicate)

		var innerIntegrity []byte
		if _, err := mu.UnmarshalFromReader(r, &innerIntegrity); err != nil {
			return nil, xerrors.Errorf("cannot unpack inner integrity digest: %w", err)
		}

		var err error
		duplicate, err = ioutil.ReadAll(r)
		if err != nil {
			return nil, xerrors.Errorf("cannot unpack inner wrapper: %w", err)
		}

		h := name.Algorithm().NewHash()
		h.Write(duplicate)
		h.Write(name)

		if !bytes.Equal(h.Sum(nil), innerIntegrity) {
			return nil, errors.New("inner integrity digest is invalid")
		}
	}

	var sensitive struct {
		Ptr *tpm2.Sensitive `tpm2:"sized"`
	}
	if _, err := mu.UnmarshalFromBytes(duplicate, &sensitive); err != nil {
		return nil, xerrors.Errorf("cannot unmarhsal sensitive: %w", err)
	}

	return sensitive.Ptr, nil
}

// SensitiveToDuplicate creates a duplication blob from the supplied sensitive structure.
// The supplied name is the name of the object associated with sensitive.
//
// If symmetricAlg is defined, an inner wrapper will be applied, first by prepending
// an inner integrity digest computed with the object's name algorithm from the sensitive
// data and its name, and then encrypting the data with innerSymKey. If innerSymKey isn't
// supplied, a random key will be created and returned.
//
// If a seed is supplied, an outer wrapper will be applied using the name algorithm and
// symmetric algorithm of parent.
func SensitiveToDuplicate(sensitive *tpm2.Sensitive, name tpm2.Name, parent *tpm2.Public, seed []byte, symmetricAlg *tpm2.SymDefObject, innerSymKey tpm2.Data) (innerSymKeyOut tpm2.Data, duplicate tpm2.Private, err error) {
	applyInnerWrapper := false
	if symmetricAlg != nil && symmetricAlg.Algorithm != tpm2.SymObjectAlgorithmNull {
		applyInnerWrapper = true
	}

	applyOuterWrapper := false
	if len(seed) > 0 {
		applyOuterWrapper = true
	}

	sensitiveSized := struct {
		Ptr *tpm2.Sensitive `tpm2:"sized"`
	}{sensitive}
	duplicate, err = mu.MarshalToBytes(sensitiveSized)
	if err != nil {
		return nil, nil, xerrors.Errorf("cannot marshal sensitive: %w", err)
	}

	if applyInnerWrapper {
		// Apply inner wrapper
		h := name.Algorithm().NewHash()
		h.Write(duplicate)
		h.Write(name)

		innerIntegrity := h.Sum(nil)

		duplicate = mu.MustMarshalToBytes(innerIntegrity, mu.RawBytes(duplicate))

		if len(innerSymKey) == 0 {
			innerSymKey = make([]byte, symmetricAlg.KeyBits.Sym/8)
			if _, err := rand.Read(innerSymKey); err != nil {
				return nil, nil, xerrors.Errorf("cannot read random bytes for key for inner wrapper: %w", err)
			}
			innerSymKeyOut = innerSymKey
		}

		if err := tpm2.CryptSymmetricEncrypt(tpm2.SymAlgorithmId(symmetricAlg.Algorithm), innerSymKey, make([]byte, symmetricAlg.Algorithm.BlockSize()), duplicate); err != nil {
			return nil, nil, xerrors.Errorf("cannot apply inner wrapper: %w", err)
		}
	}

	if applyOuterWrapper {
		// Apply outer wrapper
		var err error
		duplicate, err = ProduceOuterWrap(parent.NameAlg, &parent.Params.AsymDetail().Symmetric, name, seed, false, duplicate)
		if err != nil {
			return nil, nil, xerrors.Errorf("cannot produce outer wrapper: %w", err)
		}
	}

	return innerSymKeyOut, duplicate, nil
}
