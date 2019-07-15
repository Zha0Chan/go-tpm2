package tpm2

import (
	"crypto/rand"
	"fmt"
)

type encryptedSecret []byte

func (s encryptedSecret) SliceType() SliceType {
	return SliceTypeSizedBufferU16
}

func (t *tpmImpl) StartAuthSession(tpmKey, bind ResourceContext, sessionType SessionType, symmetric *SymDef,
	authHash AlgorithmId, auth interface{}) (ResourceContext, error) {
	if tpmKey != nil {
		return nil, InvalidParamError{"no support for salted sessions yet"}
	}
	if bind != nil {
		if err := t.checkResourceContextParam(bind); err != nil {
			return nil, err
		}
	}
	if symmetric != nil {
		return nil, InvalidParamError{"no support for parameter / response encryption yet"}
	}
	digestSize, knownDigest := digestSizes[authHash]
	if !knownDigest {
		return nil, InvalidParamError{fmt.Sprintf("unsupported authHash value %v", authHash)}
	}

	var salt []byte
	//var encryptedSalt []byte
	if tpmKey != nil {
		// TODO: Create and encrypt a salt
	} else {
		tpmKey = &permanentContext{handle: HandleNull}
	}

	var authB []byte
	if bind != nil {
		switch a := auth.(type) {
		case string:
			authB = []byte(a)
		case []byte:
			authB = a
		case nil:
		default:
			return nil, InvalidParamError{fmt.Sprintf("invalid auth value: %v", auth)}
		}
	} else {
		bind = &permanentContext{handle: HandleNull}
	}

	nonceCaller := make([]byte, digestSize)
	if _, err := rand.Read(nonceCaller); err != nil {
		return nil, fmt.Errorf("cannot read random bytes for nonceCaller: %v", err)
	}

	var sessionHandle Handle
	var nonceTPM Nonce

	if err := t.RunCommand(CommandStartAuthSession, Format{2, 5}, Format{1, 1}, tpmKey, bind,
		Nonce(nonceCaller), encryptedSecret{}, sessionType, &SymDef{Algorithm: AlgorithmNull}, authHash,
		&sessionHandle, &nonceTPM); err != nil {
		return nil, err
	}

	sessionContext := &sessionContext{handle: sessionHandle,
		hashAlg: authHash,
		nonceCaller: Nonce(nonceCaller),
		nonceTPM: nonceTPM}

	if tpmKey.Handle() != HandleNull || bind.Handle() != HandleNull {
		// TODO: concatenate salt on to authValue
		key := make([]byte, len(authB) + len(salt))
		copy(key, authB)
		copy(key[len(authB):], salt)

		sessionContext.sessionKey, _ =
			cryptKDFa(authHash, key, []byte("ATH"), []byte(nonceTPM), nonceCaller, digestSize*8)
	}

	t.addResourceContext(sessionContext)
	return sessionContext, nil
}