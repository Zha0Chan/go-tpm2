// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2_test

import (
	"bytes"
	"reflect"
	"testing"

	. "github.com/chrisccoulson/go-tpm2"
)

func TestContextSave(t *testing.T) {
	tpm := openTPMForTesting(t)
	defer closeTPM(t, tpm)

	run := func(t *testing.T, rc HandleContext, savedHandle, hierarchy Handle) {
		context, err := tpm.ContextSave(rc)
		if err != nil {
			t.Fatalf("ContextSave failed: %v", err)
		}
		if context.SavedHandle != savedHandle {
			t.Errorf("context has an unexpected handle (0x%08x)", context.SavedHandle)
		}
		if context.Hierarchy != hierarchy {
			t.Errorf("context specifies the wrong hierarchy (0x%08x)", context.Hierarchy)
		}
	}

	t.Run("TransientObject", func(t *testing.T) {
		rc := createRSASrkForTesting(t, tpm, nil)
		defer flushContext(t, tpm, rc)
		run(t, rc, Handle(0x80000000), HandleOwner)
	})
	t.Run("Session", func(t *testing.T) {
		sessionContext, err := tpm.StartAuthSession(nil, nil, SessionTypeHMAC, nil, HashAlgorithmSHA256)
		if err != nil {
			t.Fatalf("StartAuthSession failed: %v", err)
		}
		defer flushContext(t, tpm, sessionContext)
		run(t, sessionContext, sessionContext.Handle(), HandleNull)
		// Make sure that ContextSave marked the session context as not loaded, and that we get the expected error if we attempt to use it
		err = tpm.Clear(tpm.LockoutHandleContext(), &Session{Context: sessionContext})
		if err == nil {
			t.Fatalf("Expected an error")
		}
		if err.Error() != "error whilst processing handle with authorization for authContext: invalid resource context for session: not "+
			"complete and loaded" {
			t.Errorf("Unexpected error: %v", err)
		}
		handles, err := tpm.GetCapabilityHandles(HandleTypeLoadedSession.BaseHandle(), CapabilityMaxProperties)
		if err != nil {
			t.Fatalf("GetCapability failed: %v", err)
		}
		for _, h := range handles {
			if h == sessionContext.Handle() {
				t.Errorf("Session is still loaded")
			}
		}
	})
	t.Run("IncompleteSession", func(t *testing.T) {
		sessionContext, err := tpm.StartAuthSession(nil, nil, SessionTypePolicy, nil, HashAlgorithmSHA256)
		if err != nil {
			t.Fatalf("StartAuthSession failed: %v", err)
		}
		defer verifyContextFlushed(t, tpm, sessionContext)
		sessionHandle := sessionContext.Handle()
		tpm.ForgetHandleContext(sessionContext)

		sessionContext, err = tpm.GetOrCreateSessionContext(sessionHandle)
		if err != nil {
			t.Fatalf("GetOrCreateResourceContext failed: %v", err)
		}
		defer flushContext(t, tpm, sessionContext)
		_, err = tpm.ContextSave(sessionContext)
		if err == nil {
			t.Fatalf("Expected an error")
		}
		if err.Error() != "invalid saveContext parameter: unusable session HandleContext" {
			t.Errorf("Unexpected error: %v", err)
		}
	})
}

func TestContextSaveAndLoad(t *testing.T) {
	tpm := openTPMForTesting(t)
	defer closeTPM(t, tpm)

	t.Run("TransientObject", func(t *testing.T) {
		rc := createRSASrkForTesting(t, tpm, nil)
		defer flushContext(t, tpm, rc)

		context, err := tpm.ContextSave(rc)
		if err != nil {
			t.Fatalf("ContextSave failed: %v", err)
		}
		restored, err := tpm.ContextLoad(context)
		if err != nil {
			t.Fatalf("ContextLoad failed: %v", err)
		}
		defer flushContext(t, tpm, restored)

		if _, ok := restored.(ResourceContext); !ok {
			t.Fatalf("ContextLoad returned the wrong type of HandleContext")
		}
		if restored.Handle().Type() != HandleTypeTransient {
			t.Errorf("ContextLoad returned an invalid handle 0x%08x", restored.Handle())
		}
		if restored.Handle() == rc.Handle() {
			t.Errorf("ContextLoad returned a context with an unexpected handle")
		}
		if !bytes.Equal(restored.Name(), rc.Name()) {
			t.Errorf("Restored context has the wrong name")
		}
		if !reflect.DeepEqual(rc.(TestObjectResourceContext).GetPublic(), restored.(TestObjectResourceContext).GetPublic()) {
			t.Errorf("Restored context has the wrong public data")
		}

		pub, name, _, err := tpm.ReadPublic(restored.(ResourceContext))
		if err != nil {
			t.Fatalf("ReadPublic failed: %v", err)
		}
		if !bytes.Equal(name, rc.Name()) {
			t.Errorf("Restored object has the wrong name")
		}
		if !reflect.DeepEqual(pub, rc.(TestObjectResourceContext).GetPublic()) {
			t.Errorf("Restored object has the wrong public area")
		}
	})

	runSessionTest := func(t *testing.T, forget bool, tpmKey, bind ResourceContext, sessionType SessionType, hashAlg HashAlgorithmId) {
		sc, err := tpm.StartAuthSession(tpmKey, bind, sessionType, nil, hashAlg)
		if err != nil {
			t.Fatalf("StartAuthSession failed: %v", err)
		}
		defer verifyContextFlushed(t, tpm, sc)

		scData := sc.(TestSessionContext).GetScData()
		var data struct {
			handle         Handle
			name           Name
			isAudit        bool
			isExclusive    bool
			hashAlg        HashAlgorithmId
			sessionType    SessionType
			policyHMACType uint8
			isBound        bool
			boundEntity    Name
			sessionKey     []byte
			nonceCaller    Nonce
			nonceTPM       Nonce
			symmetric      *SymDef
		}
		data.handle = sc.Handle()
		data.name = sc.Name()
		data.isAudit = scData.IsAudit
		data.isExclusive = scData.IsExclusive
		data.hashAlg = scData.HashAlg
		data.sessionType = scData.SessionType
		data.policyHMACType = uint8(scData.PolicyHMACType)
		data.isBound = scData.IsBound
		data.boundEntity = scData.BoundEntity
		data.sessionKey = scData.SessionKey
		data.nonceCaller = scData.NonceCaller
		data.nonceTPM = scData.NonceTPM
		data.symmetric = scData.Symmetric

		context, err := tpm.ContextSave(sc)
		if err != nil {
			t.Fatalf("ContextSave failed: %v", err)
		}
		if forget {
			tpm.ForgetHandleContext(sc)
		}
		restored, err := tpm.ContextLoad(context)
		if err != nil {
			t.Fatalf("ContextLoad failed: %v", err)
		}
		defer flushContext(t, tpm, restored)

		if !forget && restored != sc {
			t.Errorf("Expected the same HandleContext back")
		}

		if _, ok := restored.(SessionContext); !ok {
			t.Fatalf("ContextLoad returned the wrong type of HandleContext")
		}
		if restored.Handle() != data.handle {
			t.Errorf("ContextLoad returned an invalid handle 0x%08x", restored.Handle())
		}
		if !bytes.Equal(restored.Name(), data.name) {
			t.Errorf("ContextLoad returned a handle with the wrong name")
		}
		restoredData := restored.(TestSessionContext).GetScData()
		if restoredData.IsAudit != data.isAudit {
			t.Errorf("ContextLoad returned a handle with the wrong session data")
		}
		if restoredData.IsExclusive != data.isExclusive {
			t.Errorf("ContextLoad returned a handle with the wrong session data")
		}
		if restoredData.HashAlg != data.hashAlg {
			t.Errorf("ContextLoad returned a handle with the wrong session data")
		}
		if restoredData.SessionType != data.sessionType {
			t.Errorf("ContextLoad returned a handle with the wrong session data")
		}
		if uint8(restoredData.PolicyHMACType) != data.policyHMACType {
			t.Errorf("ContextLoad returned a handle with the wrong session data")
		}
		if restoredData.IsBound != data.isBound {
			t.Errorf("ContextLoad returned a handle with the wrong session data")
		}
		if !bytes.Equal(restoredData.BoundEntity, data.boundEntity) {
			t.Errorf("ContextLoad returned a handle with the wrong session data")
		}
		if !bytes.Equal(restoredData.SessionKey, data.sessionKey) {
			t.Errorf("ContextLoad returned a handle with the wrong session data")
		}
		if !bytes.Equal(restoredData.NonceCaller, data.nonceCaller) {
			t.Errorf("ContextLoad returned a handle with the wrong session data")
		}
		if !bytes.Equal(restoredData.NonceTPM, data.nonceTPM) {
			t.Errorf("ContextLoad returned a handle with the wrong session data")
		}
		if !reflect.DeepEqual(restoredData.Symmetric, data.symmetric) {
			t.Errorf("ContextLoad returned a handle with the wrong session data")
		}

		handles, err := tpm.GetCapabilityHandles(HandleTypeLoadedSession.BaseHandle(), CapabilityMaxProperties)
		if err != nil {
			t.Fatalf("GetCapability failed: %v", err)
		}
		found := false
		for _, h := range handles {
			if h == restored.Handle() {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Session isn't loaded")
		}
	}

	t.Run("Session1", func(t *testing.T) {
		primary := createRSASrkForTesting(t, tpm, nil)
		defer flushContext(t, tpm, primary)
		runSessionTest(t, false, primary, tpm.OwnerHandleContext(), SessionTypeHMAC, HashAlgorithmSHA256)
	})
	t.Run("Session2", func(t *testing.T) {
		primary := createRSASrkForTesting(t, tpm, nil)
		defer flushContext(t, tpm, primary)
		runSessionTest(t, false, nil, primary, SessionTypeHMAC, HashAlgorithmSHA1)
	})
	t.Run("Session3", func(t *testing.T) {
		runSessionTest(t, false, nil, nil, SessionTypeHMAC, HashAlgorithmSHA256)
	})
	t.Run("Session4", func(t *testing.T) {
		runSessionTest(t, false, nil, nil, SessionTypePolicy, HashAlgorithmSHA256)
	})
	t.Run("Session5", func(t *testing.T) {
		primary := createRSASrkForTesting(t, tpm, nil)
		defer flushContext(t, tpm, primary)
		runSessionTest(t, true, primary, tpm.OwnerHandleContext(), SessionTypeHMAC, HashAlgorithmSHA256)
	})
}

func TestEvictControl(t *testing.T) {
	tpm := openTPMForTesting(t)
	defer closeTPM(t, tpm)

	run := func(t *testing.T, transient ResourceContext, persist Handle, authAuthSession *Session) {
		owner := tpm.OwnerHandleContext()
		if handle, err := tpm.GetOrCreateResourceContext(persist); err == nil {
			_, err := tpm.EvictControl(owner, handle, persist, authAuthSession)
			if err != nil {
				t.Logf("EvictControl failed whilst trying to remove a handle at the start of the test: %v", err)
			}
		}

		outContext, err := tpm.EvictControl(owner, transient, persist, authAuthSession)
		if err != nil {
			t.Fatalf("EvictControl failed: %v", err)
		}

		if outContext.Handle() != persist {
			t.Errorf("outContext has the wrong id (0x%08x)", outContext.Handle())
		}

		if !bytes.Equal(transient.Name(), outContext.Name()) {
			t.Errorf("outContext has the wrong name")
		}

		outContext2, err := tpm.EvictControl(owner, outContext, outContext.Handle(), authAuthSession)
		if err != nil {
			t.Errorf("EvictControl failed: %v", err)
		}
		if outContext2 != nil {
			t.Errorf("EvictControl should return a nil handle when evicting a persistent object")
		}

		if outContext.Handle() != HandleUnassigned {
			t.Errorf("EvictControl should set the persistent context's handle to %v", HandleUnassigned)
		}

		_, err = tpm.GetOrCreateResourceContext(persist)
		if err == nil {
			t.Fatalf("GetOrCreateResourceContext on an evicted resource should fail")
		}
		if _, ok := err.(ResourceUnavailableError); !ok {
			t.Errorf("GetOrCreateResourceContext returned an unexpected error: %v", err)
		}
	}

	t.Run("NoAuth", func(t *testing.T) {
		context := createRSASrkForTesting(t, tpm, nil)
		defer flushContext(t, tpm, context)
		run(t, context, Handle(0x8100ffff), nil)
	})
	t.Run("UsePasswordAuth", func(t *testing.T) {
		context := createRSASrkForTesting(t, tpm, nil)
		defer flushContext(t, tpm, context)
		setHierarchyAuthForTest(t, tpm, tpm.OwnerHandleContext())
		defer resetHierarchyAuth(t, tpm, tpm.OwnerHandleContext())
		run(t, context, Handle(0x8100fff0), nil)
	})
	t.Run("UseSessionAuth", func(t *testing.T) {
		context := createRSASrkForTesting(t, tpm, nil)
		defer flushContext(t, tpm, context)
		setHierarchyAuthForTest(t, tpm, tpm.OwnerHandleContext())
		defer resetHierarchyAuth(t, tpm, tpm.OwnerHandleContext())
		sessionContext, err := tpm.StartAuthSession(nil, tpm.OwnerHandleContext(), SessionTypeHMAC, nil, HashAlgorithmSHA256)
		if err != nil {
			t.Fatalf("StartAuthSession failed: %v", err)
		}
		defer flushContext(t, tpm, sessionContext)
		run(t, context, Handle(0x8100ff00), &Session{Context: sessionContext, Attrs: AttrContinueSession})
	})
}

func TestFlushContext(t *testing.T) {
	tpm := openTPMForTesting(t)
	defer closeTPM(t, tpm)

	context := createRSASrkForTesting(t, tpm, nil)
	h := context.Handle()

	handles, err := tpm.GetCapabilityHandles(h, 1)
	if err != nil {
		t.Errorf("GetCapability failed: %v", err)
	}
	if len(handles) != 1 {
		t.Errorf("GetCapability should have returned the primary key handle")
	}

	if err := tpm.FlushContext(context); err != nil {
		t.Errorf("FlushContext failed: %v", err)
	}

	handles, err = tpm.GetCapabilityHandles(h, 1)
	if err != nil {
		t.Errorf("GetCapability failed: %v", err)
	}
	if len(handles) != 0 {
		t.Errorf("FlushContext didn't flush the transient handle")
	}

	if context.Handle() != HandleUnassigned {
		t.Errorf("FlushContext should set the context's handle to %v", HandleUnassigned)
	}

	_, err = tpm.GetOrCreateResourceContext(h)
	if err == nil {
		t.Fatalf("GetOrCreateResourceContext on a flushed resource should fail")
	}
	if _, ok := err.(ResourceUnavailableError); !ok {
		t.Fatalf("GetOrCreateResourceContext on a flushed resource should fail")
	}
}
