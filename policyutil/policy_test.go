// Copyright 2023 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package policyutil_test

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"

	. "gopkg.in/check.v1"

	"github.com/canonical/go-tpm2"
	internal_testutil "github.com/canonical/go-tpm2/internal/testutil"
	"github.com/canonical/go-tpm2/mu"
	"github.com/canonical/go-tpm2/objectutil"
	. "github.com/canonical/go-tpm2/policyutil"
	"github.com/canonical/go-tpm2/testutil"
)

type mockPolicyResourceAuthorizer struct {
	authorizeFn func(tpm2.ResourceContext) (tpm2.SessionContext, error)
}

func (h *mockPolicyResourceAuthorizer) Authorize(resource tpm2.ResourceContext) (tpm2.SessionContext, error) {
	if h.authorizeFn == nil {
		return nil, errors.New("not implemented")
	}
	return h.authorizeFn(resource)
}

type policySuite struct {
	testutil.TPMTest
}

func (s *policySuite) SetUpSuite(c *C) {
	s.TPMFeatures = testutil.TPMFeatureOwnerHierarchy | testutil.TPMFeatureNV
}

var _ = Suite(&policySuite{})

type testExecutePolicyNVData struct {
	nvPub    *tpm2.NVPublic
	readAuth tpm2.ResourceContext
	contents []byte

	authSession tpm2.SessionContext

	operandB  tpm2.Operand
	offset    uint16
	operation tpm2.ArithmeticOp
}

func (s *policySuite) testPolicyNV(c *C, data *testExecutePolicyNVData) error {
	index := s.NVDefineSpace(c, tpm2.HandleOwner, nil, data.nvPub)
	c.Assert(s.TPM.NVWrite(index, index, data.contents, 0, nil), IsNil)

	readAuth := data.readAuth
	if readAuth == nil {
		readAuth = index
	}

	nvPub, _, err := s.TPM.NVReadPublic(index)
	c.Assert(err, IsNil)

	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicyNV(nvPub, data.operandB, data.offset, data.operation)
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	sessionHandle := authSessionHandle(data.authSession)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	var authorizer mockPolicyResourceAuthorizer
	authorizer.authorizeFn = func(resource tpm2.ResourceContext) (tpm2.SessionContext, error) {
		c.Check(resource.Name(), DeepEquals, readAuth.Name())
		return data.authSession, nil
	}

	tickets, err := policy.Execute(s.TPM, session, nil, &authorizer)
	c.Check(tickets, internal_testutil.LenEquals, 0)
	if err != nil {
		return err
	}
	c.Check(s.LastCommand(c).GetCommandCode(c), Equals, tpm2.CommandPolicyNV)
	_, authArea, _ := s.LastCommand(c).UnmarshalCommand(c)
	c.Assert(authArea, internal_testutil.LenEquals, 1)
	c.Check(authArea[0].SessionHandle, Equals, sessionHandle)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())

	return nil
}

func (s *policySuite) TestPolicyNV(c *C) {
	err := s.testPolicyNV(c, &testExecutePolicyNVData{
		nvPub: &tpm2.NVPublic{
			Index:   s.NextAvailableHandle(c, 0x0181f000),
			NameAlg: tpm2.HashAlgorithmSHA256,
			Attrs:   tpm2.NVTypeOrdinary.WithAttrs(tpm2.AttrNVAuthRead | tpm2.AttrNVAuthWrite | tpm2.AttrNVNoDA),
			Size:    8},
		contents:  internal_testutil.DecodeHexString(c, "0000000000001000"),
		operandB:  internal_testutil.DecodeHexString(c, "00001000"),
		offset:    4,
		operation: tpm2.OpEq})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicyNVDifferentOperand(c *C) {
	err := s.testPolicyNV(c, &testExecutePolicyNVData{
		nvPub: &tpm2.NVPublic{
			Index:   s.NextAvailableHandle(c, 0x0181f000),
			NameAlg: tpm2.HashAlgorithmSHA256,
			Attrs:   tpm2.NVTypeOrdinary.WithAttrs(tpm2.AttrNVAuthRead | tpm2.AttrNVAuthWrite | tpm2.AttrNVNoDA),
			Size:    8},
		contents:  internal_testutil.DecodeHexString(c, "0000000000001001"),
		operandB:  internal_testutil.DecodeHexString(c, "00001001"),
		offset:    4,
		operation: tpm2.OpEq})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicyNVDifferentOffset(c *C) {
	err := s.testPolicyNV(c, &testExecutePolicyNVData{
		nvPub: &tpm2.NVPublic{
			Index:   s.NextAvailableHandle(c, 0x0181f000),
			NameAlg: tpm2.HashAlgorithmSHA256,
			Attrs:   tpm2.NVTypeOrdinary.WithAttrs(tpm2.AttrNVAuthRead | tpm2.AttrNVAuthWrite | tpm2.AttrNVNoDA),
			Size:    8},
		contents:  internal_testutil.DecodeHexString(c, "0000000010000000"),
		operandB:  internal_testutil.DecodeHexString(c, "00001000"),
		offset:    2,
		operation: tpm2.OpEq})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicyNVDifferentOperation(c *C) {
	err := s.testPolicyNV(c, &testExecutePolicyNVData{
		nvPub: &tpm2.NVPublic{
			Index:   s.NextAvailableHandle(c, 0x0181f000),
			NameAlg: tpm2.HashAlgorithmSHA256,
			Attrs:   tpm2.NVTypeOrdinary.WithAttrs(tpm2.AttrNVAuthRead | tpm2.AttrNVAuthWrite | tpm2.AttrNVNoDA),
			Size:    8},
		contents:  internal_testutil.DecodeHexString(c, "0000000000001001"),
		operandB:  internal_testutil.DecodeHexString(c, "00001000"),
		offset:    4,
		operation: tpm2.OpUnsignedGT})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicyNVFails(c *C) {
	err := s.testPolicyNV(c, &testExecutePolicyNVData{
		nvPub: &tpm2.NVPublic{
			Index:   s.NextAvailableHandle(c, 0x0181f000),
			NameAlg: tpm2.HashAlgorithmSHA256,
			Attrs:   tpm2.NVTypeOrdinary.WithAttrs(tpm2.AttrNVAuthRead | tpm2.AttrNVAuthWrite | tpm2.AttrNVNoDA),
			Size:    8},
		contents:  internal_testutil.DecodeHexString(c, "0000000000001001"),
		operandB:  internal_testutil.DecodeHexString(c, "00001000"),
		offset:    4,
		operation: tpm2.OpEq})
	c.Check(err, ErrorMatches, `cannot process TPM2_PolicyNV assertion: TPM returned an error whilst executing command TPM_CC_PolicyNV: TPM_RC_POLICY \(policy failure in math operation or an invalid authPolicy value\)`)
	var e *tpm2.TPMError
	c.Assert(err, internal_testutil.ErrorAs, &e)
	c.Check(e, DeepEquals, &tpm2.TPMError{Command: tpm2.CommandPolicyNV, Code: tpm2.ErrorPolicy})
}

func (s *policySuite) TestPolicyNVDifferentAuth(c *C) {
	err := s.testPolicyNV(c, &testExecutePolicyNVData{
		nvPub: &tpm2.NVPublic{
			Index:   s.NextAvailableHandle(c, 0x0181f000),
			NameAlg: tpm2.HashAlgorithmSHA256,
			Attrs:   tpm2.NVTypeOrdinary.WithAttrs(tpm2.AttrNVOwnerRead | tpm2.AttrNVAuthWrite | tpm2.AttrNVNoDA),
			Size:    8},
		readAuth:  s.TPM.OwnerHandleContext(),
		contents:  internal_testutil.DecodeHexString(c, "0000000000001000"),
		operandB:  internal_testutil.DecodeHexString(c, "00001000"),
		offset:    4,
		operation: tpm2.OpEq})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicyNVWithAuthSession(c *C) {
	authSession := s.StartAuthSession(c, nil, nil, tpm2.SessionTypeHMAC, nil, tpm2.HashAlgorithmSHA256)
	handle := authSession.Handle()

	err := s.testPolicyNV(c, &testExecutePolicyNVData{
		nvPub: &tpm2.NVPublic{
			Index:   s.NextAvailableHandle(c, 0x0181f000),
			NameAlg: tpm2.HashAlgorithmSHA256,
			Attrs:   tpm2.NVTypeOrdinary.WithAttrs(tpm2.AttrNVAuthRead | tpm2.AttrNVAuthWrite | tpm2.AttrNVNoDA),
			Size:    8},
		contents:    internal_testutil.DecodeHexString(c, "0000000000001000"),
		authSession: authSession.WithAttrs(tpm2.AttrContinueSession),
		operandB:    internal_testutil.DecodeHexString(c, "00001000"),
		offset:      4,
		operation:   tpm2.OpEq})
	c.Check(err, IsNil)
	c.Check(s.TPM.DoesHandleExist(handle), internal_testutil.IsTrue)
}

func (s *policySuite) TestPolicyNVMissingIndex(c *C) {
	nvPub := &tpm2.NVPublic{
		Index:   0x0181f000,
		NameAlg: tpm2.HashAlgorithmSHA256,
		Attrs:   tpm2.NVTypeOrdinary.WithAttrs(tpm2.AttrNVAuthRead | tpm2.AttrNVAuthWrite | tpm2.AttrNVNoDA | tpm2.AttrNVWritten),
		Size:    8}

	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicyNV(nvPub, nil, 0, tpm2.OpEq)
	_, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	_, err = policy.Execute(s.TPM, session, nil, nil)
	c.Check(err, ErrorMatches, "cannot process TPM2_PolicyNV assertion: cannot create nvIndex context: a resource at handle 0x0181f000 is not available on the TPM")
}

type testExecutePolicySecretData struct {
	authObject Named
	policyRef  tpm2.Nonce
	params     *PolicyExecuteParams

	expectedCpHash     tpm2.Digest
	expectedExpiration int32

	expectedFlush bool

	authSession tpm2.SessionContext
}

func (s *policySuite) testPolicySecret(c *C, data *testExecutePolicySecretData) error {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicySecret(data.authObject, data.policyRef)
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	sessionHandle := authSessionHandle(data.authSession)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	var authObjectHandle tpm2.Handle
	var authorizer mockPolicyResourceAuthorizer
	authorizer.authorizeFn = func(resource tpm2.ResourceContext) (tpm2.SessionContext, error) {
		c.Check(resource.Name(), DeepEquals, data.authObject.Name())
		authObjectHandle = resource.Handle()
		return data.authSession, nil
	}

	tickets, err := policy.Execute(s.TPM, session, data.params, &authorizer)
	if data.expectedExpiration < 0 {
		c.Assert(tickets, internal_testutil.LenEquals, 1)
		c.Check(tickets[0].AuthName, DeepEquals, data.authObject.Name())
		c.Check(tickets[0].PolicyRef, DeepEquals, data.policyRef)
		c.Check(tickets[0].CpHash, DeepEquals, data.expectedCpHash)
		c.Check(tickets[0].Ticket.Tag, Equals, tpm2.TagAuthSecret)
		c.Check(tickets[0].Ticket.Hierarchy, Equals, tpm2.HandleOwner)
	} else {
		c.Check(tickets, internal_testutil.LenEquals, 0)
	}
	if err != nil {
		return err
	}

	var policyCommand *testutil.CommandRecordC
	if data.expectedFlush {
		commands := s.CommandLog()
		c.Assert(commands, internal_testutil.LenGreaterEquals, 2)
		policyCommand = commands[len(commands)-2]
		c.Check(s.TPM.DoesHandleExist(authObjectHandle), internal_testutil.IsFalse)
	} else {
		policyCommand = s.LastCommand(c)
		c.Check(s.TPM.DoesHandleExist(authObjectHandle), internal_testutil.IsTrue)
	}
	c.Check(policyCommand.GetCommandCode(c), Equals, tpm2.CommandPolicySecret)
	_, authArea, cpBytes := policyCommand.UnmarshalCommand(c)
	c.Assert(authArea, internal_testutil.LenEquals, 1)
	c.Check(authArea[0].SessionHandle, Equals, sessionHandle)

	var nonceTPM tpm2.Nonce
	var cpHashA tpm2.Digest
	var policyRef tpm2.Nonce
	var expiration int32
	_, err = mu.UnmarshalFromBytes(cpBytes, &nonceTPM, &cpHashA, &policyRef, &expiration)
	c.Check(err, IsNil)
	c.Check(cpHashA, DeepEquals, data.expectedCpHash)
	c.Check(expiration, Equals, data.expectedExpiration)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())

	return nil
}

func (s *policySuite) TestPolicySecret(c *C) {
	err := s.testPolicySecret(c, &testExecutePolicySecretData{
		authObject: s.TPM.OwnerHandleContext(),
		policyRef:  []byte("foo")})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySecretNoPolicyRef(c *C) {
	err := s.testPolicySecret(c, &testExecutePolicySecretData{
		authObject: s.TPM.OwnerHandleContext()})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySecretWithParams(c *C) {
	h := crypto.SHA256.New()
	io.WriteString(h, "bar")
	cpHash := h.Sum(nil)

	params := &PolicySecretParams{
		AuthName:   s.TPM.OwnerHandleContext().Name(),
		PolicyRef:  []byte("foo"),
		CpHash:     CommandParameterDigest(tpm2.HashAlgorithmSHA256, cpHash),
		Expiration: 100}
	err := s.testPolicySecret(c, &testExecutePolicySecretData{
		authObject:         s.TPM.OwnerHandleContext(),
		policyRef:          params.PolicyRef,
		params:             &PolicyExecuteParams{SecretParams: []*PolicySecretParams{params}},
		expectedCpHash:     cpHash,
		expectedExpiration: params.Expiration})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySecretWithNonMatchingParams1(c *C) {
	h := crypto.SHA256.New()
	io.WriteString(h, "bar")

	params := &PolicySecretParams{
		AuthName:   s.TPM.OwnerHandleContext().Name(),
		CpHash:     CommandParameterDigest(tpm2.HashAlgorithmSHA256, h.Sum(nil)),
		Expiration: 100}
	err := s.testPolicySecret(c, &testExecutePolicySecretData{
		authObject: s.TPM.OwnerHandleContext(),
		policyRef:  []byte("foo"),
		params:     &PolicyExecuteParams{SecretParams: []*PolicySecretParams{params}}})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySecretWithNonMatchingParams2(c *C) {
	h := crypto.SHA256.New()
	io.WriteString(h, "bar")

	params := &PolicySecretParams{
		AuthName:   s.TPM.EndorsementHandleContext().Name(),
		PolicyRef:  []byte("foo"),
		CpHash:     CommandParameterDigest(tpm2.HashAlgorithmSHA256, h.Sum(nil)),
		Expiration: 100}
	err := s.testPolicySecret(c, &testExecutePolicySecretData{
		authObject: s.TPM.OwnerHandleContext(),
		policyRef:  params.PolicyRef,
		params:     &PolicyExecuteParams{SecretParams: []*PolicySecretParams{params}}})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySecretWithRequestedTicket(c *C) {
	h := crypto.SHA256.New()
	io.WriteString(h, "bar")
	cpHash := h.Sum(nil)

	params := &PolicySecretParams{
		AuthName:   s.TPM.OwnerHandleContext().Name(),
		PolicyRef:  []byte("foo"),
		CpHash:     CommandParameterDigest(tpm2.HashAlgorithmSHA256, cpHash),
		Expiration: -200}
	err := s.testPolicySecret(c, &testExecutePolicySecretData{
		authObject:         s.TPM.OwnerHandleContext(),
		policyRef:          params.PolicyRef,
		params:             &PolicyExecuteParams{SecretParams: []*PolicySecretParams{params}},
		expectedCpHash:     cpHash,
		expectedExpiration: params.Expiration})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySecretWithSession(c *C) {
	err := s.testPolicySecret(c, &testExecutePolicySecretData{
		authObject:  s.TPM.OwnerHandleContext(),
		policyRef:   []byte("foo"),
		authSession: s.StartAuthSession(c, nil, nil, tpm2.SessionTypeHMAC, nil, tpm2.HashAlgorithmSHA256)})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySecretWithWithTransient(c *C) {
	object := s.CreateStoragePrimaryKeyRSA(c)
	err := s.testPolicySecret(c, &testExecutePolicySecretData{
		authObject: object,
		policyRef:  []byte("foo"),
		params: &PolicyExecuteParams{
			Resources: &PolicyResources{Loaded: []tpm2.ResourceContext{object}}}})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySecretWithWithTransientSaved(c *C) {
	object := s.CreateStoragePrimaryKeyRSA(c)

	saved, err := SaveAndFlushResource(s.TPM, object)
	c.Assert(err, IsNil)

	err = s.testPolicySecret(c, &testExecutePolicySecretData{
		authObject: saved.Name,
		policyRef:  []byte("foo"),
		params: &PolicyExecuteParams{
			Resources: &PolicyResources{Saved: []*SavedContext{saved}}},
		expectedFlush: true})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySecretWithWithTransientLoadable(c *C) {
	parent := s.CreateStoragePrimaryKeyRSA(c)
	priv, pub, _, _, _, err := s.TPM.Create(parent, nil, testutil.NewRSAStorageKeyTemplate(), nil, nil, nil)
	c.Assert(err, IsNil)

	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicySecret(pub, []byte("foo"))
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	parentHandle := parent.Handle()

	authorizations := 0
	var authObjectHandle tpm2.Handle
	var authorizer mockPolicyResourceAuthorizer
	authorizer.authorizeFn = func(resource tpm2.ResourceContext) (tpm2.SessionContext, error) {
		switch authorizations {
		case 0:
			c.Check(resource.Name(), DeepEquals, parent.Name())
		case 1:
			c.Check(resource.Name(), DeepEquals, pub.Name())
			authObjectHandle = resource.Handle()
		default:
			return nil, errors.New("unexpected")
		}
		authorizations += 1
		return nil, nil
	}

	params := &PolicyExecuteParams{
		Resources: &PolicyResources{
			Loaded: []tpm2.ResourceContext{parent},
			Unloaded: []*LoadableObject{
				{
					ParentName: parent.Name(),
					Private:    priv,
					Public:     pub,
				},
			},
		},
	}
	tickets, err := policy.Execute(s.TPM, session, params, &authorizer)
	c.Check(tickets, internal_testutil.LenEquals, 0)

	c.Check(s.TPM.DoesHandleExist(authObjectHandle), internal_testutil.IsFalse)
	c.Check(s.TPM.DoesHandleExist(parentHandle), internal_testutil.IsTrue)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())
}

func (s *policySuite) TestPolicySecretFail(c *C) {
	s.TPM.OwnerHandleContext().SetAuthValue([]byte("1234"))

	err := s.testPolicySecret(c, &testExecutePolicySecretData{
		authObject: s.TPM.OwnerHandleContext(),
		policyRef:  []byte("foo")})
	c.Check(err, ErrorMatches, `cannot process TPM2_PolicySecret assertion: TPM returned an error for session 1 whilst executing command TPM_CC_PolicySecret: TPM_RC_BAD_AUTH \(authorization failure without DA implications\)`)
	var e *tpm2.TPMSessionError
	c.Assert(err, internal_testutil.ErrorAs, &e)
	c.Check(e, DeepEquals, &tpm2.TPMSessionError{TPMError: &tpm2.TPMError{Command: tpm2.CommandPolicySecret, Code: tpm2.ErrorBadAuth}, Index: 1})
}

func (s *policySuite) TestPolicySecretMissingResource(c *C) {
	object := s.CreateStoragePrimaryKeyRSA(c)

	saved, err := SaveAndFlushResource(s.TPM, object)
	c.Assert(err, IsNil)

	err = s.testPolicySecret(c, &testExecutePolicySecretData{
		authObject: saved.Name,
		policyRef:  []byte("foo")})
	c.Check(err, ErrorMatches, `cannot process TPM2_PolicySecret assertion: cannot create authObject context: missing resource with name 0x([[:xdigit:]]{68})`)

	var rnfe ResourceNotFoundError
	c.Check(err, internal_testutil.ErrorAs, &rnfe)
}

func (s *policySuite) TestPolicySecretTicket(c *C) {
	authObject := s.TPM.OwnerHandleContext()
	policyRef := tpm2.Nonce("foo")

	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicySecret(authObject, policyRef)
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	var authorizer mockPolicyResourceAuthorizer
	authorizer.authorizeFn = func(resource tpm2.ResourceContext) (tpm2.SessionContext, error) {
		c.Check(resource.Name(), DeepEquals, authObject.Name())
		return nil, nil
	}

	params := &PolicyExecuteParams{
		SecretParams: []*PolicySecretParams{{
			AuthName:   authObject.Name(),
			PolicyRef:  policyRef,
			Expiration: -1000}}}

	tickets, err := policy.Execute(s.TPM, session, params, &authorizer)
	c.Check(err, IsNil)
	c.Check(tickets, internal_testutil.LenEquals, 1)

	c.Check(s.TPM.PolicyRestart(session), IsNil)

	params = &PolicyExecuteParams{Tickets: tickets}

	tickets, err = policy.Execute(s.TPM, session, params, nil)
	c.Check(err, IsNil)
	c.Check(tickets, internal_testutil.LenEquals, 0)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())
}

type testExecutePolicySignedData struct {
	authKey   *tpm2.Public
	policyRef tpm2.Nonce

	params *PolicyExecuteParams

	signer          crypto.Signer
	includeNonceTPM bool
	cpHashA         CpHash
	expiration      int32
	signerOpts      crypto.SignerOpts
}

func (s *policySuite) testPolicySigned(c *C, data *testExecutePolicySignedData) error {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicySigned(data.authKey, data.policyRef)
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	var nonceTPM tpm2.Nonce
	if data.includeNonceTPM {
		nonceTPM = session.NonceTPM()
	}

	auth, err := NewPolicyAuthorization(data.authKey, data.policyRef, session.HashAlg(), nonceTPM, data.cpHashA, data.expiration)
	c.Assert(err, IsNil)
	c.Check(auth.Sign(rand.Reader, data.signer, data.signerOpts), IsNil)

	var params *PolicyExecuteParams
	if data.params != nil {
		params = &(*data.params)
	}
	if params == nil {
		params = new(PolicyExecuteParams)
	}

	params.Authorizations = append(params.Authorizations, auth)

	tickets, err := policy.Execute(s.TPM, session, params, nil)
	if data.expiration < 0 && err == nil {
		expectedCpHash, err := data.cpHashA.Digest(session.HashAlg())
		c.Check(err, IsNil)

		c.Assert(tickets, internal_testutil.LenEquals, 1)
		c.Check(tickets[0].AuthName, DeepEquals, data.authKey.Name())
		c.Check(tickets[0].PolicyRef, DeepEquals, data.policyRef)
		c.Check(tickets[0].CpHash, DeepEquals, expectedCpHash)
		c.Check(tickets[0].Ticket.Tag, Equals, tpm2.TagAuthSigned)
		c.Check(tickets[0].Ticket.Hierarchy, Equals, tpm2.HandleOwner)
	} else {
		c.Check(tickets, internal_testutil.LenEquals, 0)
	}
	if err != nil {
		return err
	}

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())

	return nil
}

func (s *policySuite) TestPolicySigned(c *C) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c.Assert(err, IsNil)

	pubKey, err := objectutil.NewECCPublicKey(&key.PublicKey)
	c.Assert(err, IsNil)

	err = s.testPolicySigned(c, &testExecutePolicySignedData{
		authKey:    pubKey,
		policyRef:  []byte("foo"),
		signer:     key,
		signerOpts: tpm2.HashAlgorithmSHA256})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySignedNoPolicyRef(c *C) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c.Assert(err, IsNil)

	pubKey, err := objectutil.NewECCPublicKey(&key.PublicKey)
	c.Assert(err, IsNil)

	err = s.testPolicySigned(c, &testExecutePolicySignedData{
		authKey:         pubKey,
		signer:          key,
		includeNonceTPM: true,
		signerOpts:      tpm2.HashAlgorithmSHA256})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySignedIncludeTPMNonce(c *C) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c.Assert(err, IsNil)

	pubKey, err := objectutil.NewECCPublicKey(&key.PublicKey)
	c.Assert(err, IsNil)

	err = s.testPolicySigned(c, &testExecutePolicySignedData{
		authKey:    pubKey,
		policyRef:  []byte("foo"),
		signer:     key,
		signerOpts: tpm2.HashAlgorithmSHA256})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySignedWithCpHash(c *C) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c.Assert(err, IsNil)

	pubKey, err := objectutil.NewECCPublicKey(&key.PublicKey)
	c.Assert(err, IsNil)

	err = s.testPolicySigned(c, &testExecutePolicySignedData{
		authKey:    pubKey,
		policyRef:  []byte("foo"),
		signer:     key,
		cpHashA:    CommandParameters(tpm2.CommandLoad, []Named{tpm2.Name{0x40, 0x00, 0x00, 0x01}}, tpm2.Private{1, 2, 3, 4}, mu.Sized(objectutil.NewRSAStorageKeyTemplate())),
		signerOpts: tpm2.HashAlgorithmSHA256})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySignedWithExpiration(c *C) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c.Assert(err, IsNil)

	pubKey, err := objectutil.NewECCPublicKey(&key.PublicKey)
	c.Assert(err, IsNil)

	err = s.testPolicySigned(c, &testExecutePolicySignedData{
		authKey:    pubKey,
		policyRef:  []byte("foo"),
		signer:     key,
		expiration: 100,
		signerOpts: tpm2.HashAlgorithmSHA256})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySignedWithRequestedTicket(c *C) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c.Assert(err, IsNil)

	pubKey, err := objectutil.NewECCPublicKey(&key.PublicKey)
	c.Assert(err, IsNil)

	err = s.testPolicySigned(c, &testExecutePolicySignedData{
		authKey:    pubKey,
		policyRef:  []byte("foo"),
		signer:     key,
		cpHashA:    CommandParameters(tpm2.CommandLoad, []Named{tpm2.Name{0x40, 0x00, 0x00, 0x01}}, tpm2.Private{1, 2, 3, 4}, mu.Sized(objectutil.NewRSAStorageKeyTemplate())),
		expiration: -100,
		signerOpts: tpm2.HashAlgorithmSHA256})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySignedWithInvalidSignature(c *C) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c.Assert(err, IsNil)

	pubKey, err := objectutil.NewECCPublicKey(&key.PublicKey)
	c.Assert(err, IsNil)

	key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c.Assert(err, IsNil)

	err = s.testPolicySigned(c, &testExecutePolicySignedData{
		authKey:    pubKey,
		policyRef:  []byte("foo"),
		signer:     key,
		signerOpts: tpm2.HashAlgorithmSHA256})
	c.Check(err, ErrorMatches, `cannot process TPM2_PolicySigned assertion: TPM returned an error for parameter 5 whilst executing command TPM_CC_PolicySigned: TPM_RC_SIGNATURE \(the signature is not valid\)`)
	var e *tpm2.TPMParameterError
	c.Assert(err, internal_testutil.ErrorAs, &e)
	c.Check(e, DeepEquals, &tpm2.TPMParameterError{TPMError: &tpm2.TPMError{Command: tpm2.CommandPolicySigned, Code: tpm2.ErrorSignature}, Index: 5})
}

func (s *policySuite) TestPolicySignedWithNonMatchingAuth(c *C) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c.Assert(err, IsNil)

	pubKey, err := objectutil.NewECCPublicKey(&key.PublicKey)
	c.Assert(err, IsNil)

	err = s.testPolicySigned(c, &testExecutePolicySignedData{
		authKey:   pubKey,
		policyRef: []byte("foo"),
		params: &PolicyExecuteParams{
			Authorizations: []*PolicyAuthorization{
				{AuthName: pubKey.Name(), PolicyRef: []byte("bar")}}},
		signer:     key,
		signerOpts: tpm2.HashAlgorithmSHA256})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicySignedWithTicket(c *C) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	c.Assert(err, IsNil)

	authKey, err := objectutil.NewECCPublicKey(&key.PublicKey)
	c.Assert(err, IsNil)

	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicySigned(authKey, nil)
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	auth, err := NewPolicyAuthorization(authKey, nil, session.HashAlg(), session.NonceTPM(), nil, -100)
	c.Assert(err, IsNil)
	c.Check(auth.Sign(rand.Reader, key, tpm2.HashAlgorithmSHA256), IsNil)

	params := &PolicyExecuteParams{Authorizations: []*PolicyAuthorization{auth}}

	tickets, err := policy.Execute(s.TPM, session, params, nil)
	c.Check(err, IsNil)
	c.Check(tickets, internal_testutil.LenEquals, 1)

	c.Check(s.TPM.PolicyRestart(session), IsNil)

	params = &PolicyExecuteParams{Tickets: tickets}

	tickets, err = policy.Execute(s.TPM, session, params, nil)
	c.Check(err, IsNil)
	c.Check(tickets, internal_testutil.LenEquals, 0)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())
}

func (s *policySuite) TestPolicyAuthValue(c *C) {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicyAuthValue()
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	tickets, err := policy.Execute(s.TPM, session, nil, nil)
	c.Check(err, IsNil)
	c.Check(tickets, internal_testutil.LenEquals, 0)

	// TPM2_PolicyPassword and TPM2_PolicyAuthValue have the same digest, so make sure
	// we executed the correct command.
	c.Check(s.LastCommand(c).GetCommandCode(c), Equals, tpm2.CommandPolicyAuthValue)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())
}

func (s *policySuite) testPolicyCommandCode(c *C, code tpm2.CommandCode) {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicyCommandCode(code)
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	tickets, err := policy.Execute(s.TPM, session, nil, nil)
	c.Check(err, IsNil)
	c.Check(tickets, internal_testutil.LenEquals, 0)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())
}

func (s *policySuite) TestPolicyCommandCodeNVChangeAuth(c *C) {
	s.testPolicyCommandCode(c, tpm2.CommandNVChangeAuth)
}

func (s *policySuite) TestPolicyCommandCodeUnseal(c *C) {
	s.testPolicyCommandCode(c, tpm2.CommandUnseal)
}

type testExecutePolicyCounterTimerData struct {
	operandB  tpm2.Operand
	offset    uint16
	operation tpm2.ArithmeticOp
}

func (s *policySuite) testPolicyCounterTimer(c *C, data *testExecutePolicyCounterTimerData) error {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicyCounterTimer(data.operandB, data.offset, data.operation)
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	tickets, err := policy.Execute(s.TPM, session, nil, nil)
	if err != nil {
		return err
	}
	c.Check(tickets, internal_testutil.LenEquals, 0)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())

	return nil
}

func (s *policySuite) TestPolicyCounterTimer1(c *C) {
	c.Skip("test fails in github")

	timeInfo, err := s.TPM.ReadClock()
	c.Assert(err, IsNil)

	operandB := make(tpm2.Operand, binary.Size(uint64(0)))
	binary.BigEndian.PutUint64(operandB, timeInfo.ClockInfo.Clock)

	err = s.testPolicyCounterTimer(c, &testExecutePolicyCounterTimerData{
		operandB:  operandB,
		offset:    8,
		operation: tpm2.OpUnsignedGT})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicyCounterTimer2(c *C) {
	timeInfo, err := s.TPM.ReadClock()
	c.Assert(err, IsNil)

	operandB := make(tpm2.Operand, binary.Size(uint32(0)))
	binary.BigEndian.PutUint32(operandB, timeInfo.ClockInfo.RestartCount)

	err = s.testPolicyCounterTimer(c, &testExecutePolicyCounterTimerData{
		operandB:  operandB,
		offset:    20,
		operation: tpm2.OpEq})
	c.Check(err, IsNil)
}

func (s *policySuite) TestPolicyCounterTimerFails(c *C) {
	timeInfo, err := s.TPM.ReadClock()
	c.Assert(err, IsNil)

	operandB := make(tpm2.Operand, binary.Size(uint64(0)))
	binary.BigEndian.PutUint64(operandB, timeInfo.ClockInfo.Clock)

	err = s.testPolicyCounterTimer(c, &testExecutePolicyCounterTimerData{
		operandB:  operandB,
		offset:    8,
		operation: tpm2.OpUnsignedLT})
	c.Check(err, ErrorMatches, `cannot process TPM2_PolicyCounterTimer assertion: TPM returned an error whilst executing command TPM_CC_PolicyCounterTimer: TPM_RC_POLICY \(policy failure in math operation or an invalid authPolicy value\)`)
	var e *tpm2.TPMError
	c.Assert(err, internal_testutil.ErrorAs, &e)
	c.Check(e, DeepEquals, &tpm2.TPMError{Command: tpm2.CommandPolicyCounterTimer, Code: tpm2.ErrorPolicy})
}

func (s *policySuite) testPolicyCpHash(c *C, cpHashA CpHash) {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicyCpHash(cpHashA)
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	tickets, err := policy.Execute(s.TPM, session, nil, nil)
	c.Check(err, IsNil)
	c.Check(tickets, internal_testutil.LenEquals, 0)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())
}

func (s *policySuite) TestPolicyCpHash1(c *C) {
	s.testPolicyCpHash(c, CommandParameters(tpm2.CommandLoad, []Named{tpm2.Name{0x40, 0x00, 0x00, 0x01}}, tpm2.Private{1, 2, 3, 4}, mu.Sized(objectutil.NewRSAStorageKeyTemplate())))
}

func (s *policySuite) TestPolicyCpHash2(c *C) {
	s.testPolicyCpHash(c, CommandParameters(tpm2.CommandLoad, []Named{tpm2.Name{0x40, 0x00, 0x00, 0x01}}, tpm2.Private{1, 2, 3, 4, 5}, mu.Sized(objectutil.NewRSAStorageKeyTemplate())))
}

func (s *policySuite) TestPolicyCpHashMultipleDigests(c *C) {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA1, tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicyCpHash(CommandParameters(tpm2.CommandLoad, []Named{tpm2.Name{0x40, 0x00, 0x00, 0x01}}, tpm2.Private{1, 2, 3, 4}, mu.Sized(objectutil.NewRSAStorageKeyTemplate())))
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	tickets, err := policy.Execute(s.TPM, session, nil, nil)
	c.Check(err, IsNil)
	c.Check(tickets, internal_testutil.LenEquals, 0)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[1].Digest())
}

func (s *policySuite) TestPolicyCpHashMissingDigest(c *C) {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA1)
	pc.RootBranch().PolicyCpHash(CommandParameters(tpm2.CommandLoad, []Named{tpm2.Name{0x40, 0x00, 0x00, 0x01}}, tpm2.Private{1, 2, 3, 4}, mu.Sized(objectutil.NewRSAStorageKeyTemplate())))
	_, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	_, err = policy.Execute(s.TPM, session, nil, nil)
	c.Check(err, ErrorMatches, "cannot process TPM2_PolicyCpHash assertion: missing digest for session algorithm")
	c.Check(err, internal_testutil.ErrorIs, ErrMissingDigest)
}

func (s *policySuite) testPolicyNameHash(c *C, nameHash NameHash) {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicyNameHash(nameHash)
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	tickets, err := policy.Execute(s.TPM, session, nil, nil)
	c.Check(err, IsNil)
	c.Check(tickets, internal_testutil.LenEquals, 0)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())
}

func (s *policySuite) TestPolicyNameHash1(c *C) {
	s.testPolicyNameHash(c, CommandHandles(tpm2.Name{0x40, 0x00, 0x00, 0x01}))
}

func (s *policySuite) TestPolicyNameHash2(c *C) {
	s.testPolicyNameHash(c, CommandHandles(tpm2.Name{0x40, 0x00, 0x00, 0x0b}))
}

func (s *policySuite) TestPolicyNameHashMultipleDigests(c *C) {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA1, tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicyNameHash(CommandHandles(tpm2.Name{0x40, 0x00, 0x00, 0x01}))
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	tickets, err := policy.Execute(s.TPM, session, nil, nil)
	c.Check(err, IsNil)
	c.Check(tickets, internal_testutil.LenEquals, 0)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[1].Digest())
}

func (s *policySuite) TestPolicyNameHashMissingDigest(c *C) {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA1)
	pc.RootBranch().PolicyNameHash(CommandHandles(tpm2.Name{0x40, 0x00, 0x00, 0x01}))
	_, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	_, err = policy.Execute(s.TPM, session, nil, nil)
	c.Check(err, ErrorMatches, "cannot process TPM2_PolicyNameHash assertion: missing digest for session algorithm")
	c.Check(err, internal_testutil.ErrorIs, ErrMissingDigest)
}

func (s *policySuite) testPolicyPCR(c *C, values tpm2.PCRValues) error {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicyPCR(values)
	expectedDigests, policy, err := pc.Policy()

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	tickets, err := policy.Execute(s.TPM, session, nil, nil)
	if err != nil {
		return err
	}
	c.Check(tickets, internal_testutil.LenEquals, 0)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())

	return nil
}

func (s *policySuite) TestPolicyPCR(c *C) {
	_, values, err := s.TPM.PCRRead(tpm2.PCRSelectionList{{Hash: tpm2.HashAlgorithmSHA256, Select: []int{4, 7}}})
	c.Assert(err, IsNil)

	c.Check(s.testPolicyPCR(c, values), IsNil)
}

func (s *policySuite) TestPolicyPCRDifferentDigestAndSelection(c *C) {
	_, values, err := s.TPM.PCRRead(tpm2.PCRSelectionList{
		{Hash: tpm2.HashAlgorithmSHA1, Select: []int{4}},
		{Hash: tpm2.HashAlgorithmSHA256, Select: []int{7}}})
	c.Assert(err, IsNil)

	c.Check(s.testPolicyPCR(c, values), IsNil)
}

func (s *policySuite) TestPolicyPCRFails(c *C) {
	values := tpm2.PCRValues{
		tpm2.HashAlgorithmSHA256: {
			0: internal_testutil.DecodeHexString(c, "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")}}
	err := s.testPolicyPCR(c, values)
	c.Check(err, ErrorMatches, `cannot process TPM2_PolicyPCR assertion: TPM returned an error for parameter 1 whilst executing command TPM_CC_PolicyPCR: TPM_RC_VALUE \(value is out of range or is not correct for the context\)`)
	var e *tpm2.TPMParameterError
	c.Assert(err, internal_testutil.ErrorAs, &e)
	c.Check(e, DeepEquals, &tpm2.TPMParameterError{TPMError: &tpm2.TPMError{Command: tpm2.CommandPolicyPCR, Code: tpm2.ErrorValue}, Index: 1})
}

type testExecutePolicyDuplicationSelectData struct {
	object        Named
	newParent     Named
	includeObject bool
}

func (s *policySuite) testPolicyDuplicationSelect(c *C, data *testExecutePolicyDuplicationSelectData) {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicyDuplicationSelect(data.object, data.newParent, data.includeObject)
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	tickets, err := policy.Execute(s.TPM, session, nil, nil)
	c.Check(err, IsNil)
	c.Check(tickets, internal_testutil.LenEquals, 0)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())
}

func (s *policySuite) TestPolicyDuplicationSelect(c *C) {
	h := crypto.SHA256.New()
	io.WriteString(h, "foo")
	object := tpm2.Name(mu.MustMarshalToBytes(tpm2.HashAlgorithmSHA256, mu.Raw(h.Sum(nil))))

	h = crypto.SHA256.New()
	io.WriteString(h, "bar")
	newParent := tpm2.Name(mu.MustMarshalToBytes(tpm2.HashAlgorithmSHA256, mu.Raw(h.Sum(nil))))

	s.testPolicyDuplicationSelect(c, &testExecutePolicyDuplicationSelectData{
		object:        object,
		newParent:     newParent,
		includeObject: true})
}

func (s *policySuite) TestPolicyDuplicationSelectNoIncludeObject(c *C) {
	h := crypto.SHA256.New()
	io.WriteString(h, "foo")
	object := tpm2.Name(mu.MustMarshalToBytes(tpm2.HashAlgorithmSHA256, mu.Raw(h.Sum(nil))))

	h = crypto.SHA256.New()
	io.WriteString(h, "bar")
	newParent := tpm2.Name(mu.MustMarshalToBytes(tpm2.HashAlgorithmSHA256, mu.Raw(h.Sum(nil))))

	s.testPolicyDuplicationSelect(c, &testExecutePolicyDuplicationSelectData{
		object:        object,
		newParent:     newParent,
		includeObject: false})
}

func (s *policySuite) TestPolicyDuplicationSelectDifferentNames(c *C) {
	h := crypto.SHA256.New()
	io.WriteString(h, "bar")
	object := tpm2.Name(mu.MustMarshalToBytes(tpm2.HashAlgorithmSHA256, mu.Raw(h.Sum(nil))))

	h = crypto.SHA256.New()
	io.WriteString(h, "foo")
	newParent := tpm2.Name(mu.MustMarshalToBytes(tpm2.HashAlgorithmSHA256, mu.Raw(h.Sum(nil))))

	s.testPolicyDuplicationSelect(c, &testExecutePolicyDuplicationSelectData{
		object:        object,
		newParent:     newParent,
		includeObject: true})
}

func (s *policySuite) TestPolicyPassword(c *C) {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicyPassword()
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	tickets, err := policy.Execute(s.TPM, session, nil, nil)
	c.Check(err, IsNil)
	c.Check(tickets, internal_testutil.LenEquals, 0)

	// TPM2_PolicyPassword and TPM2_PolicyAuthValue have the same digest, so make sure
	// we executed the correct command.
	c.Check(s.LastCommand(c).GetCommandCode(c), Equals, tpm2.CommandPolicyPassword)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())
}

func (s *policySuite) testPolicyNvWritten(c *C, writtenSet bool) {
	pc := ComputePolicy(tpm2.HashAlgorithmSHA256)
	pc.RootBranch().PolicyNvWritten(writtenSet)
	expectedDigests, policy, err := pc.Policy()
	c.Assert(err, IsNil)

	session := s.StartAuthSession(c, nil, nil, tpm2.SessionTypePolicy, nil, tpm2.HashAlgorithmSHA256)

	tickets, err := policy.Execute(s.TPM, session, nil, nil)
	c.Check(err, IsNil)
	c.Check(tickets, internal_testutil.LenEquals, 0)

	digest, err := s.TPM.PolicyGetDigest(session)
	c.Check(err, IsNil)
	c.Check(digest, DeepEquals, expectedDigests[0].Digest())
}

func (s *policySuite) TestPolicyNvWrittenFalse(c *C) {
	s.testPolicyNvWritten(c, false)
}

func (s *policySuite) TestPolicyNvWrittenTrue(c *C) {
	s.testPolicyNvWritten(c, true)
}