// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2

import (
	"reflect"
	"unsafe"

	"github.com/canonical/go-tpm2/mu"
)

// This file contains types defined in section 11 (Algorithm Parameters
// and Structures) in part 2 of the library spec.

// 11.1) Symmetric

// SymKeyBitsU is a union type that corresponds to the TPMU_SYM_KEY_BITS type
// and is used to specify symmetric encryption key sizes. The selector type is
// AlgorithmId. Mapping of selector values to fields is as follows:
//  - AlgorithmAES: Sym
//  - AlgorithmSM4: Sym
//  - AlgorithmCamellia: Sym
//  - AlgorithmXOR: XOR
//  - AlgorithmNull: none
type SymKeyBitsU struct {
	Sym uint16
	XOR HashAlgorithmId
}

func (b *SymKeyBitsU) Select(selector reflect.Value) interface{} {
	switch selector.Convert(reflect.TypeOf(AlgorithmId(0))).Interface().(AlgorithmId) {
	case AlgorithmAES:
		fallthrough
	case AlgorithmSM4:
		fallthrough
	case AlgorithmCamellia:
		return &b.Sym
	case AlgorithmXOR:
		return &b.XOR
	case AlgorithmNull:
		return mu.NilUnionValue
	default:
		return nil
	}
}

// SymModeU is a union type that corresponds to the TPMU_SYM_MODE type. The selector
// type is AlgorithmId. The mapping of selector values to fields is as follows:
//  - AlgorithmAES: Sym
//  - AlgorithmSM4: Sym
//  - AlgorithmCamellia: Sym
//  - AlgorithmXOR: none
//  - AlgorithmNull: none
type SymModeU struct {
	Sym SymModeId
}

func (m *SymModeU) Select(selector reflect.Value) interface{} {
	switch selector.Convert(reflect.TypeOf(AlgorithmId(0))).Interface().(AlgorithmId) {
	case AlgorithmAES:
		fallthrough
	case AlgorithmSM4:
		fallthrough
	case AlgorithmCamellia:
		return &m.Sym
	case AlgorithmXOR:
		fallthrough
	case AlgorithmNull:
		return mu.NilUnionValue
	default:
		return nil
	}
}

// SymDef corresponds to the TPMT_SYM_DEF type, and is used to select the algorithm
// used for parameter encryption.
type SymDef struct {
	Algorithm SymAlgorithmId // Symmetric algorithm
	KeyBits   *SymKeyBitsU   // Symmetric key size
	Mode      *SymModeU      // Symmetric mode
}

// SymDefObject corresponds to the TPMT_SYM_DEF_OBJECT type, and is used to define an
// object's symmetric algorithm.
type SymDefObject struct {
	Algorithm SymObjectAlgorithmId // Symmetric algorithm
	KeyBits   *SymKeyBitsU         // Symmetric key size
	Mode      *SymModeU            // Symmetric mode
}

// SymKey corresponds to the TPM2B_SYM_KEY type.
type SymKey []byte

// SymCipherParams corresponds to the TPMS_SYMCIPHER_PARMS type, and contains the
// parameters for a symmetric object.
type SymCipherParams struct {
	Sym SymDefObject
}

// Label corresponds to the TPM2B_LABEL type.
type Label []byte

// Derive corresponds to the TPMS_DERIVE type.
type Derive struct {
	Label   Label
	Context Label
}

// SensitiveCreate corresponds to the TPMS_SENSITIVE_CREATE type and is used to define
// the values to be placed in the sensitive area of a created object.
type SensitiveCreate struct {
	UserAuth Auth          // Authorization value
	Data     SensitiveData // Secret data
}

// SensitiveData corresponds to the TPM2B_SENSITIVE_DATA type.
type SensitiveData []byte

// SchemeHash corresponds to the TPMS_SCHEME_HASH type, and is used for schemes that only
// require a hash algorithm to complete their definition.
type SchemeHash struct {
	HashAlg HashAlgorithmId // Hash algorithm used to digest the message
}

// SchemeECDAA corresponds to the TPMS_SCHEME_ECDAA type.
type SchemeECDAA struct {
	HashAlg HashAlgorithmId // Hash algorithm used to digest the message
	Count   uint16
}

// KeyedHashSchemeId corresponds to the TPMI_ALG_KEYEDHASH_SCHEME type
type KeyedHashSchemeId AlgorithmId

// SchemeHMAC corresponds to the TPMS_SCHEME_HMAC type.
type SchemeHMAC SchemeHash

// SchemeXOR corresponds to the TPMS_SCHEME_XOR type, and is used to define the XOR encryption
// scheme.
type SchemeXOR struct {
	HashAlg HashAlgorithmId // Hash algorithm used to digest the message
	KDF     KDFAlgorithmId  // Hash algorithm used for the KDF
}

// SchemeKeyedHashU is a union type that corresponds to the TPMU_SCHEME_KEYED_HASH type.
// The selector type is KeyedHashSchemeId. The mapping of selector values to fields is
// as follows:
//  - KeyedHashSchemeHMAC: HMAC
//  - KeyedHashSchemeXOR: XOR
//  - KeyedHashSchemeNull: none
type SchemeKeyedHashU struct {
	HMAC *SchemeHMAC
	XOR  *SchemeXOR
}

func (d *SchemeKeyedHashU) Select(selector reflect.Value) interface{} {
	switch selector.Interface().(KeyedHashSchemeId) {
	case KeyedHashSchemeHMAC:
		return &d.HMAC
	case KeyedHashSchemeXOR:
		return &d.XOR
	case KeyedHashSchemeNull:
		return mu.NilUnionValue
	default:
		return nil
	}
}

// KeyedHashScheme corresponds to the TPMT_KEYEDHASH_SCHEME type.
type KeyedHashScheme struct {
	Scheme  KeyedHashSchemeId // Scheme selector
	Details *SchemeKeyedHashU // Scheme specific parameters
}

// 11.2 Assymetric

// 11.2.1 Signing Schemes

type SigSchemeRSASSA SchemeHash
type SigSchemeRSAPSS SchemeHash
type SigSchemeECDSA SchemeHash
type SigSchemeECDAA SchemeECDAA
type SigSchemeSM2 SchemeHash
type SigSchemeECSCHNORR SchemeHash

// SigSchemeU is a union type that corresponds to the TPMU_SIG_SCHEME type. The
// selector type is SigSchemeId. The mapping of selector value to fields is as follows:
//  - SigSchemeAlgRSASSA: RSASSA
//  - SigSchemeAlgRSAPSS: RSAPSS
//  - SigSchemeAlgECDSA: ECDSA
//  - SigSchemeAlgECDAA: ECDAA
//  - SigSchemeAlgSM2: SM2
//  - SigSchemeAlgECSCHNORR: ECSCHNORR
//  - SigSchemeAlgHMAC: HMAC
//  - SigSchemeAlgNull: none
type SigSchemeU struct {
	RSASSA    *SigSchemeRSASSA
	RSAPSS    *SigSchemeRSAPSS
	ECDSA     *SigSchemeECDSA
	ECDAA     *SigSchemeECDAA
	SM2       *SigSchemeSM2
	ECSCHNORR *SigSchemeECSCHNORR
	HMAC      *SchemeHMAC
}

func (s *SigSchemeU) Select(selector reflect.Value) interface{} {
	switch selector.Interface().(SigSchemeId) {
	case SigSchemeAlgRSASSA:
		return &s.RSASSA
	case SigSchemeAlgRSAPSS:
		return &s.RSAPSS
	case SigSchemeAlgECDSA:
		return &s.ECDSA
	case SigSchemeAlgECDAA:
		return &s.ECDAA
	case SigSchemeAlgSM2:
		return &s.SM2
	case SigSchemeAlgECSCHNORR:
		return &s.ECSCHNORR
	case SigSchemeAlgHMAC:
		return &s.HMAC
	case SigSchemeAlgNull:
		return mu.NilUnionValue
	default:
		return nil
	}
}

// Any returns the underlying value as *SchemeHash. Note that if more than
// one field is set, it will return the first set field as *SchemeHash.
func (s SigSchemeU) Any() *SchemeHash {
	switch {
	case s.RSASSA != nil:
		return (*SchemeHash)(unsafe.Pointer(s.RSASSA))
	case s.RSAPSS != nil:
		return (*SchemeHash)(unsafe.Pointer(s.RSAPSS))
	case s.ECDSA != nil:
		return (*SchemeHash)(unsafe.Pointer(s.ECDSA))
	case s.ECDAA != nil:
		return (*SchemeHash)(unsafe.Pointer(s.ECDAA))
	case s.SM2 != nil:
		return (*SchemeHash)(unsafe.Pointer(s.SM2))
	case s.ECSCHNORR != nil:
		return (*SchemeHash)(unsafe.Pointer(s.ECSCHNORR))
	case s.HMAC != nil:
		return (*SchemeHash)(unsafe.Pointer(s.HMAC))
	default:
		return nil
	}
}

// SigScheme corresponds to the TPMT_SIG_SCHEME type.
type SigScheme struct {
	Scheme  SigSchemeId // Scheme selector
	Details *SigSchemeU // Scheme specific parameters
}

// 11.2.2 Encryption Schemes

type EncSchemeRSAES Empty
type EncSchemeOAEP SchemeHash

type KeySchemeECDH SchemeHash
type KeySchemeECMQV SchemeHash

// 11.2.3 Key Derivation Schemes

type SchemeMGF1 SchemeHash
type SchemeKDF1_SP800_56A SchemeHash
type SchemeKDF2 SchemeHash
type SchemeKDF1_SP800_108 SchemeHash

// KDFSchemeU is a union type that corresponds to the TPMU_KDF_SCHEME
// type. The selector type is KDFAlgorithmId. The mapping of selector
// value to field is as follows:
//  - KDFAlgorithmMGF1: MGF1
//  - KDFAlgorithmKDF1_SP800_56A: KDF1_SP800_56A
//  - KDFAlgorithmKDF2: KDF2
//  - KDFAlgorithmKDF1_SP800_108: KDF1_SP800_108
//  - KDFAlgorithmNull: none
type KDFSchemeU struct {
	MGF1           *SchemeMGF1
	KDF1_SP800_56A *SchemeKDF1_SP800_56A
	KDF2           *SchemeKDF2
	KDF1_SP800_108 *SchemeKDF1_SP800_108
}

func (s *KDFSchemeU) Select(selector reflect.Value) interface{} {
	switch selector.Interface().(KDFAlgorithmId) {
	case KDFAlgorithmMGF1:
		return &s.MGF1
	case KDFAlgorithmKDF1_SP800_56A:
		return &s.KDF1_SP800_56A
	case KDFAlgorithmKDF2:
		return &s.KDF2
	case KDFAlgorithmKDF1_SP800_108:
		return &s.KDF1_SP800_108
	case KDFAlgorithmNull:
		return mu.NilUnionValue
	default:
		return nil
	}
}

// KDFScheme corresponds to the TPMT_KDF_SCHEME type.
type KDFScheme struct {
	Scheme  KDFAlgorithmId // Scheme selector
	Details *KDFSchemeU    // Scheme specific parameters.
}

// AsymSchemeId corresponds to the TPMI_ALG_ASYM_SCHEME type
type AsymSchemeId AlgorithmId

// AsymSchemeU is a union type that corresponds to the TPMU_ASYM_SCHEME type. The
// selector type is AsymSchemeId. The mapping of selector values to fields is as follows:
//  - AsymSchemeRSASSA: RSASSA
//  - AsymSchemeRSAES: RSAES
//  - AsymSchemeRSAPSS: RSAPSS
//  - AsymSchemeOAEP: OAEP
//  - AsymSchemeECDSA: ECDSA
//  - AsymSchemeECDH: ECDH
//  - AsymSchemeECDAA: ECDAA
//  - AsymSchemeSM2: SM2
//  - AsymSchemeECSCHNORR: ECSCHNORR
//  - AsymSchemeECMQV: ECMQV
//  - AsymSchemeNull: none
type AsymSchemeU struct {
	RSASSA    *SigSchemeRSASSA
	RSAES     *EncSchemeRSAES
	RSAPSS    *SigSchemeRSAPSS
	OAEP      *EncSchemeOAEP
	ECDSA     *SigSchemeECDSA
	ECDH      *KeySchemeECDH
	ECDAA     *SigSchemeECDAA
	SM2       *SigSchemeSM2
	ECSCHNORR *SigSchemeECSCHNORR
	ECMQV     *KeySchemeECMQV
}

func (s *AsymSchemeU) Select(selector reflect.Value) interface{} {
	switch selector.Convert(reflect.TypeOf(AsymSchemeId(0))).Interface().(AsymSchemeId) {
	case AsymSchemeRSASSA:
		return &s.RSASSA
	case AsymSchemeRSAES:
		return &s.RSAES
	case AsymSchemeRSAPSS:
		return &s.RSAPSS
	case AsymSchemeOAEP:
		return &s.OAEP
	case AsymSchemeECDSA:
		return &s.ECDSA
	case AsymSchemeECDH:
		return &s.ECDH
	case AsymSchemeECDAA:
		return &s.ECDAA
	case AsymSchemeSM2:
		return &s.SM2
	case AsymSchemeECSCHNORR:
		return &s.ECSCHNORR
	case AsymSchemeECMQV:
		return &s.ECMQV
	case AsymSchemeNull:
		return mu.NilUnionValue
	default:
		return nil
	}
}

// Any returns the underlying value as *SchemeHash. Note that if more than one field
// is set, it will return the first set field as *SchemeHash.
func (s AsymSchemeU) Any() *SchemeHash {
	switch {
	case s.RSASSA != nil:
		return (*SchemeHash)(unsafe.Pointer(s.RSASSA))
	case s.RSAPSS != nil:
		return (*SchemeHash)(unsafe.Pointer(s.RSAPSS))
	case s.OAEP != nil:
		return (*SchemeHash)(unsafe.Pointer(s.OAEP))
	case s.ECDSA != nil:
		return (*SchemeHash)(unsafe.Pointer(s.ECDSA))
	case s.ECDH != nil:
		return (*SchemeHash)(unsafe.Pointer(s.ECDH))
	case s.ECDAA != nil:
		return (*SchemeHash)(unsafe.Pointer(s.ECDAA))
	case s.SM2 != nil:
		return (*SchemeHash)(unsafe.Pointer(s.SM2))
	case s.ECSCHNORR != nil:
		return (*SchemeHash)(unsafe.Pointer(s.ECSCHNORR))
	case s.ECMQV != nil:
		return (*SchemeHash)(unsafe.Pointer(s.ECMQV))
	default:
		return nil
	}
}

// AsymScheme corresponds to the TPMT_ASYM_SCHEME type.
type AsymScheme struct {
	Scheme  AsymSchemeId // Scheme selector
	Details *AsymSchemeU // Scheme specific parameters
}

// 11.2.4 RSA

// RSASchemeId corresponds to the TPMI_ALG_RSA_SCHEME type.
type RSASchemeId AsymSchemeId

// RSAScheme corresponds to the TPMT_RSA_SCHEME type.
type RSAScheme struct {
	Scheme  RSASchemeId  // Scheme selector
	Details *AsymSchemeU // Scheme specific parameters.
}

// PublicKeyRSA corresponds to the TPM2B_PUBLIC_KEY_RSA type.
type PublicKeyRSA []byte

// PrivateKeyRSA corresponds to the TPM2B_PRIVATE_KEY_RSA type.
type PrivateKeyRSA []byte

// 11.2.5 ECC

// ECCParameter corresponds to the TPM2B_ECC_PARAMETER type.
type ECCParameter []byte

// ECCPoint corresponds to the TPMS_ECC_POINT type, and contains the coordinates
// that define an ECC point.
type ECCPoint struct {
	X ECCParameter // X coordinate
	Y ECCParameter // Y coordinate
}

// ECCSchemeId corresponds to the TPMI_ALG_ECC_SCHEME type.
type ECCSchemeId AsymSchemeId

// ECCScheme corresponds to the TPMT_ECC_SCHEME type.
type ECCScheme struct {
	Scheme  ECCSchemeId  // Scheme selector
	Details *AsymSchemeU // Scheme specific parameters.
}

// 11.3 Signatures

// SignatureRSA corresponds to the TPMS_SIGNATURE_RSA type.
type SignatureRSA struct {
	Hash HashAlgorithmId // Hash algorithm used to digest the message
	Sig  PublicKeyRSA    // Signature, which is the same size as the public key
}

// SignatureECC corresponds to the TPMS_SIGNATURE_ECC type.
type SignatureECC struct {
	Hash       HashAlgorithmId // Hash is the digest algorithm used in the signature process
	SignatureR ECCParameter
	SignatureS ECCParameter
}

type SignatureRSASSA SignatureRSA
type SignatureRSAPSS SignatureRSA
type SignatureECDSA SignatureECC
type SignatureECDAA SignatureECC
type SignatureSM2 SignatureECC
type SignatureECSCHNORR SignatureECC

// SignatureU is a union type that corresponds to TPMU_SIGNATURE. The selector
// type is SigSchemeId. The mapping of selector values to fields is as follows:
//  - SigSchemeAlgRSASSA: RSASSA
//  - SigSchemeAlgRSAPSS: RSAPSS
//  - SigSchemeAlgECDSA: ECDSA
//  - SigSchemeAlgECDAA: ECDAA
//  - SigSchemeAlgSM2: SM2
//  - SigSchemeAlgECSCHNORR: ECSCHNORR
//  - SigSchemeAlgHMAC: HMAC
//  - SigSchemeAlgNull: none
type SignatureU struct {
	RSASSA    *SignatureRSASSA
	RSAPSS    *SignatureRSAPSS
	ECDSA     *SignatureECDSA
	ECDAA     *SignatureECDAA
	SM2       *SignatureSM2
	ECSCHNORR *SignatureECSCHNORR
	HMAC      *TaggedHash
}

func (s *SignatureU) Select(selector reflect.Value) interface{} {
	switch selector.Interface().(SigSchemeId) {
	case SigSchemeAlgRSASSA:
		return &s.RSASSA
	case SigSchemeAlgRSAPSS:
		return &s.RSAPSS
	case SigSchemeAlgECDSA:
		return &s.ECDSA
	case SigSchemeAlgECDAA:
		return &s.ECDAA
	case SigSchemeAlgSM2:
		return &s.SM2
	case SigSchemeAlgECSCHNORR:
		return &s.ECSCHNORR
	case SigSchemeAlgHMAC:
		return &s.HMAC
	case SigSchemeAlgNull:
		return mu.NilUnionValue
	default:
		return nil
	}
}

// Any returns the underlying value as *SchemeHash. Note that if more than one field
// is set, it will return the first set field as *SchemeHash.
func (s SignatureU) Any() *SchemeHash {
	switch {
	case s.RSASSA != nil:
		return (*SchemeHash)(unsafe.Pointer(s.RSASSA))
	case s.RSAPSS != nil:
		return (*SchemeHash)(unsafe.Pointer(s.RSAPSS))
	case s.ECDSA != nil:
		return (*SchemeHash)(unsafe.Pointer(s.ECDSA))
	case s.ECDAA != nil:
		return (*SchemeHash)(unsafe.Pointer(s.ECDAA))
	case s.SM2 != nil:
		return (*SchemeHash)(unsafe.Pointer(s.SM2))
	case s.ECSCHNORR != nil:
		return (*SchemeHash)(unsafe.Pointer(s.ECSCHNORR))
	case s.HMAC != nil:
		return (*SchemeHash)(unsafe.Pointer(s.HMAC))
	default:
		return nil
	}
}

// Signature corresponds to the TPMT_SIGNATURE type. It is returned by the attestation
// commands, and is a parameter for TPMContext.VerifySignature and TPMContext.PolicySigned.
type Signature struct {
	SigAlg    SigSchemeId // Signature algorithm
	Signature *SignatureU // Actual signature
}

// 11.4) Key/Secret Exchange

// EncryptedSecret corresponds to the TPM2B_ENCRYPTED_SECRET type.
type EncryptedSecret []byte
