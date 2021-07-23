// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2_test

import (
	"encoding/binary"
	"fmt"

	. "gopkg.in/check.v1"

	. "github.com/canonical/go-tpm2"
	"github.com/canonical/go-tpm2/testutil"
)

type resourcesSuite struct {
	testutil.TPMTest
}

func (s *resourcesSuite) SetUpTest(c *C) {
	s.TPMFeatures = testutil.TPMFeatureOwnerHierarchy | testutil.TPMFeatureNV
	s.TPMTest.SetUpTest(c)
}

var _ = Suite(&resourcesSuite{})

type testCreateObjectResourceContextFromTPMData struct {
	handle Handle
	public *Public
	name   Name
}

func (s *resourcesSuite) testCreateObjectResourceContextFromTPM(c *C, data *testCreateObjectResourceContextFromTPMData) {
	rc, err := s.TPM.CreateResourceContextFromTPM(data.handle)
	c.Assert(err, IsNil)
	c.Assert(rc, NotNil)
	c.Check(rc.Handle(), Equals, data.handle)
	c.Check(rc.Name(), DeepEquals, data.name)
	c.Assert(rc, testutil.ConvertibleTo, &ObjectContext{})
	c.Check(rc.(*ObjectContext).GetPublic(), DeepEquals, data.public)
}

func (s *resourcesSuite) TestCreateResourceContextFromTPMTransient(c *C) {
	rc := s.CreateStoragePrimaryKeyRSA(c)
	s.testCreateObjectResourceContextFromTPM(c, &testCreateObjectResourceContextFromTPMData{
		handle: rc.Handle(),
		public: rc.(*ObjectContext).GetPublic(),
		name:   rc.Name()})
}

func (s *resourcesSuite) TestCreateResourceContextFromTPMPersistent(c *C) {
	rc := s.CreateStoragePrimaryKeyRSA(c)
	rc = s.EvictControl(c, HandleOwner, rc, 0x81000008)
	s.testCreateObjectResourceContextFromTPM(c, &testCreateObjectResourceContextFromTPMData{
		handle: rc.Handle(),
		public: rc.(*ObjectContext).GetPublic(),
		name:   rc.Name()})
}

func (s *resourcesSuite) TestCreateResourceContextFromTPMNV(c *C) {
	pub := NVPublic{
		Index:   0x018100ff,
		NameAlg: HashAlgorithmSHA256,
		Attrs:   NVTypeOrdinary.WithAttrs(AttrNVAuthRead | AttrNVAuthWrite),
		Size:    8}
	rc := s.NVDefineSpace(c, s.TPM.OwnerHandleContext(), nil, &pub)

	rc2, err := s.TPM.CreateResourceContextFromTPM(rc.Handle())
	c.Assert(err, IsNil)
	c.Assert(rc, NotNil)
	c.Check(rc2.Handle(), Equals, rc.Handle())
	c.Check(rc2.Name(), DeepEquals, rc.Name())
	c.Assert(rc, testutil.ConvertibleTo, &NvIndexContext{})
	c.Check(rc2.(*NvIndexContext).GetPublic(), DeepEquals, &pub)
}

func (s *resourcesSuite) testCreateResourceContextFromTPMUnavailable(c *C, handle Handle) {
	rc, err := s.TPM.CreateResourceContextFromTPM(handle)
	c.Check(rc, IsNil)
	c.Check(err, ErrorMatches, fmt.Sprintf("a resource at handle 0x%08x is not available on the TPM", handle))
}

func (s *resourcesSuite) TestCreateResourceContextFromTPMUnavailableTransient(c *C) {
	s.testCreateResourceContextFromTPMUnavailable(c, 0x80000000)
}

func (s *resourcesSuite) TestCreateResourceContextFromTPMUnavailablePersistent(c *C) {
	s.testCreateResourceContextFromTPMUnavailable(c, 0x8100ff00)
}

func (s *resourcesSuite) TestCreateResourceContextFromTPMUnavailableNV(c *C) {
	s.testCreateResourceContextFromTPMUnavailable(c, 0x018100ff)
}

func (s *resourcesSuite) TestCreateResourceContextFromTPMPanicsForWrongType(c *C) {
	c.Check(func() { s.TPM.CreateResourceContextFromTPM(HandleOwner) }, PanicMatches, "invalid handle type")
}

func (s *resourcesSuite) testCreatePartialHandleContext(c *C, handle Handle) {
	hc := CreatePartialHandleContext(handle)
	c.Assert(hc, NotNil)
	c.Check(hc.Handle(), Equals, handle)

	name := make(Name, binary.Size(Handle(0)))
	binary.BigEndian.PutUint32(name, uint32(handle))
	c.Check(hc.Name(), DeepEquals, name)
}

func (s *resourcesSuite) TestCreatePartialHandleContextSession(c *C) {
	session := s.StartAuthSession(c, nil, nil, SessionTypeHMAC, nil, HashAlgorithmSHA256)
	s.testCreatePartialHandleContext(c, session.Handle())
}

func (s *resourcesSuite) TestCreatePartialHandleContextTransient(c *C) {
	rc := s.CreateStoragePrimaryKeyRSA(c)
	s.testCreatePartialHandleContext(c, rc.Handle())
}

func (s *resourcesSuite) TestCreatePartialHandleContextForWrongType(c *C) {
	c.Check(func() { CreatePartialHandleContext(0x81000000) }, PanicMatches, "invalid handle type")
}

type testCreateObjectHandleContextFromBytesData struct {
	b      []byte
	handle Handle
	public *Public
	name   Name
}

func (s *resourcesSuite) testCreateObjectHandleContextFromBytes(c *C, data *testCreateObjectHandleContextFromBytesData) {
	context, n, err := CreateHandleContextFromBytes(data.b)
	c.Assert(err, IsNil)
	c.Check(n, Equals, len(data.b))
	c.Assert(context, NotNil)

	c.Check(context.Handle(), Equals, data.handle)
	c.Check(context.Name(), DeepEquals, data.name)
	c.Assert(context, testutil.ConvertibleTo, &ObjectContext{})
	c.Check(context.(*ObjectContext).GetPublic(), DeepEquals, data.public)
}

func (s *resourcesSuite) TestCreateHandleContextFromBytesTransient(c *C) {
	rc := s.CreateStoragePrimaryKeyRSA(c)
	s.testCreateObjectHandleContextFromBytes(c, &testCreateObjectHandleContextFromBytesData{
		b:      rc.SerializeToBytes(),
		handle: rc.Handle(),
		public: rc.(*ObjectContext).GetPublic(),
		name:   rc.Name()})
}

func (s *resourcesSuite) TestCreateHandleContextFromBytesPersistent(c *C) {
	rc := s.CreateStoragePrimaryKeyRSA(c)
	rc = s.EvictControl(c, HandleOwner, rc, 0x81000008)
	s.testCreateObjectHandleContextFromBytes(c, &testCreateObjectHandleContextFromBytesData{
		b:      rc.SerializeToBytes(),
		handle: rc.Handle(),
		public: rc.(*ObjectContext).GetPublic(),
		name:   rc.Name()})
}

func (s *resourcesSuite) TestCreateHandleContextFromBytesNV(c *C) {
	pub := NVPublic{
		Index:   0x018100ff,
		NameAlg: HashAlgorithmSHA256,
		Attrs:   NVTypeOrdinary.WithAttrs(AttrNVAuthRead | AttrNVAuthWrite),
		Size:    8}
	rc := s.NVDefineSpace(c, s.TPM.OwnerHandleContext(), nil, &pub)
	b := rc.SerializeToBytes()

	rc2, n, err := CreateHandleContextFromBytes(b)
	c.Assert(err, IsNil)
	c.Check(n, Equals, len(b))
	c.Assert(rc2, NotNil)

	c.Check(rc2.Handle(), Equals, rc.Handle())
	c.Check(rc2.Name(), DeepEquals, rc.Name())
	c.Assert(rc2, testutil.ConvertibleTo, &NvIndexContext{})
	c.Check(rc2.(*NvIndexContext).GetPublic(), DeepEquals, &pub)
}

func (s *resourcesSuite) TestCreateHandleContextFromBytesSession(c *C) {
	session := s.StartAuthSession(c, nil, nil, SessionTypeHMAC, nil, HashAlgorithmSHA256)
	b := session.SerializeToBytes()

	session2, n, err := CreateHandleContextFromBytes(b)
	c.Assert(err, IsNil)
	c.Check(n, Equals, len(b))
	c.Assert(session2, NotNil)

	c.Check(session2.Handle(), Equals, session.Handle())
	c.Check(session2.Name(), DeepEquals, session.Name())
	c.Assert(session2, testutil.ConvertibleTo, &TestSessionContext{})

	data := session.(*TestSessionContext).Data()
	c.Check(Canonicalize(&data), IsNil)
	c.Check(session2.(*TestSessionContext).Data(), DeepEquals, data)
}

type testCreateResourceContextFromTPMWithSessionData struct {
	handle Handle
	name   Name
}

func (s *resourcesSuite) testCreateResourceContextFromTPMWithSession(c *C, data *testCreateResourceContextFromTPMWithSessionData) {
	session := s.StartAuthSession(c, nil, nil, SessionTypeHMAC, nil, HashAlgorithmSHA256)

	rc, err := s.TPM.CreateResourceContextFromTPM(data.handle, session.WithAttrs(AttrContinueSession|AttrAudit))
	c.Assert(err, IsNil)
	c.Assert(rc, NotNil)
	c.Check(rc.Handle(), Equals, data.handle)
	c.Check(rc.Name(), DeepEquals, data.name)

	_, authArea, _ := s.LastCommand(c).UnmarshalCommand(c)
	c.Assert(authArea, HasLen, 1)
	c.Check(authArea[0].SessionHandle, Equals, session.Handle())
}

func (s *resourcesSuite) TestCreateResourceContextFromTPMWithSessionTransient(c *C) {
	rc := s.CreateStoragePrimaryKeyRSA(c)
	s.testCreateResourceContextFromTPMWithSession(c, &testCreateResourceContextFromTPMWithSessionData{
		handle: rc.Handle(),
		name:   rc.Name()})
}

func (s *resourcesSuite) TestCreateResourceContextFromTPMWithSessionNV(c *C) {
	pub := NVPublic{
		Index:   0x018100ff,
		NameAlg: HashAlgorithmSHA256,
		Attrs:   NVTypeOrdinary.WithAttrs(AttrNVAuthRead | AttrNVAuthWrite),
		Size:    8}
	rc := s.NVDefineSpace(c, s.TPM.OwnerHandleContext(), nil, &pub)
	s.testCreateResourceContextFromTPMWithSession(c, &testCreateResourceContextFromTPMWithSessionData{
		handle: rc.Handle(),
		name:   rc.Name()})
}

func (s *resourcesSuite) TestCreateNVIndexResourceContextFromPublic(c *C) {
	pub := NVPublic{
		Index:   0x018100ff,
		NameAlg: HashAlgorithmSHA256,
		Attrs:   NVTypeOrdinary.WithAttrs(AttrNVAuthRead | AttrNVAuthWrite),
		Size:    8}
	rc, err := CreateNVIndexResourceContextFromPublic(&pub)
	c.Assert(err, IsNil)
	c.Assert(rc, NotNil)
	c.Check(rc.Handle(), Equals, pub.Index)

	name, err := pub.Name()
	c.Check(err, IsNil)

	c.Check(rc.Name(), DeepEquals, name)
	c.Check(rc, testutil.ConvertibleTo, &NvIndexContext{})
	c.Check(rc.(*NvIndexContext).GetPublic(), DeepEquals, &pub)
}

func (s *resourcesSuite) TestCreateObjectResourceContextFromPublic(c *C) {
	rc := s.CreateStoragePrimaryKeyRSA(c)

	pub, _, _, err := s.TPM.ReadPublic(rc)
	c.Assert(err, IsNil)

	rc2, err := CreateObjectResourceContextFromPublic(rc.Handle(), pub)
	c.Assert(err, IsNil)
	c.Assert(rc2, NotNil)
	c.Check(rc2.Handle(), Equals, rc.Handle())
	c.Check(rc2.Name(), DeepEquals, rc.Name())
	c.Check(rc2, testutil.ConvertibleTo, &ObjectContext{})
	c.Check(rc2.(*ObjectContext).GetPublic(), DeepEquals, pub)
}

func (s *resourcesSuite) TestSessionContextSetAttrs(c *C) {
	session := s.StartAuthSession(c, nil, nil, SessionTypeHMAC, nil, HashAlgorithmSHA256)

	session.SetAttrs(AttrContinueSession)
	c.Check(session.(*TestSessionContext).Attrs(), Equals, AttrContinueSession)
}

func (s *resourcesSuite) TestSessionContextWithAttrs(c *C) {
	session := s.StartAuthSession(c, nil, nil, SessionTypeHMAC, nil, HashAlgorithmSHA256)

	session2 := session.WithAttrs(AttrAudit)
	c.Check(session2.Handle(), Equals, session.Handle())
	c.Check(session2.Name(), DeepEquals, session.Name())
	c.Check(session.(*TestSessionContext).Attrs(), Equals, SessionAttributes(0))
	c.Check(session2.(*TestSessionContext).Attrs(), Equals, AttrAudit)
}

func (s *resourcesSuite) TestSessionContextIncludeAttrs(c *C) {
	session := s.StartAuthSession(c, nil, nil, SessionTypeHMAC, nil, HashAlgorithmSHA256)
	session.SetAttrs(AttrContinueSession)

	session2 := session.IncludeAttrs(AttrResponseEncrypt)
	c.Check(session2.Handle(), Equals, session.Handle())
	c.Check(session2.Name(), DeepEquals, session.Name())
	c.Check(session.(*TestSessionContext).Attrs(), Equals, AttrContinueSession)
	c.Check(session2.(*TestSessionContext).Attrs(), Equals, AttrContinueSession|AttrResponseEncrypt)
}

func (s *resourcesSuite) TestSessionContextExcludeAttrs(c *C) {
	session := s.StartAuthSession(c, nil, nil, SessionTypeHMAC, nil, HashAlgorithmSHA256)
	session.SetAttrs(AttrAudit | AttrContinueSession | AttrCommandEncrypt)

	session2 := session.ExcludeAttrs(AttrAudit)
	c.Check(session2.Handle(), Equals, session.Handle())
	c.Check(session2.Name(), DeepEquals, session.Name())
	c.Check(session.(*TestSessionContext).Attrs(), Equals, AttrAudit|AttrContinueSession|AttrCommandEncrypt)
	c.Check(session2.(*TestSessionContext).Attrs(), Equals, AttrContinueSession|AttrCommandEncrypt)
}
