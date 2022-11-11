// Copyright 2022 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package tpm2_test

import (
	"errors"

	. "gopkg.in/check.v1"

	. "github.com/canonical/go-tpm2"
	internal_testutil "github.com/canonical/go-tpm2/internal/testutil"
	"github.com/canonical/go-tpm2/mu"
)

type commandSuite struct{}

var _ = Suite(&commandSuite{})

func (s *commandSuite) TestMarshalCommandPacketNoSessions(c *C) {
	cpBytes := internal_testutil.DecodeHexString(c, "00204355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd8650000000010000b")
	p, err := MarshalCommandPacket(CommandStartAuthSession, HandleList{HandleNull, 0x80000000}, nil, cpBytes)
	c.Check(err, IsNil)

	expected := internal_testutil.DecodeHexString(c, "80010000003b00000176400000078000000000204355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd8650000000010000b")
	c.Check(p, DeepEquals, CommandPacket(expected))
}

func (s *commandSuite) TestMarshalCommandPacketWithSessions(c *C) {
	authArea := []AuthCommand{
		{
			SessionHandle:     HandlePW,
			SessionAttributes: AttrContinueSession,
			HMAC:              []byte("foo"),
		},
		{
			SessionHandle:     0x02000001,
			Nonce:             internal_testutil.DecodeHexString(c, "4355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865"),
			SessionAttributes: AttrResponseEncrypt,
			HMAC:              internal_testutil.DecodeHexString(c, "042aea10a0f14f2d391373599be69d53a75dde9951fc3d3cd10b6100aa7a9f24"),
		}}
	p, err := MarshalCommandPacket(CommandUnseal, HandleList{0x80000001}, authArea, nil)
	c.Check(err, IsNil)

	expected := internal_testutil.DecodeHexString(c, "8002000000670000015e8000000100000055400000090000010003666f6f0200000100204355a46b19d348dc2f57c046f8ef63d4538ebb936000f3c9ee954a27460dd865400020042aea10a0f14f2d391373599be69d53a75dde9951fc3d3cd10b6100aa7a9f24")
	c.Check(p, DeepEquals, CommandPacket(expected))
}

func (s *commandSuite) TestUnmarshalResponsePacketTooSmall(c *C) {
	p := ResponsePacket(internal_testutil.DecodeHexString(c, "80010000000a000000"))
	_, _, _, err := p.Unmarshal(nil)
	c.Check(err, ErrorMatches, `cannot unmarshal header: cannot unmarshal argument whilst processing element of type tpm2.ResponseCode: unexpected EOF

=== BEGIN STACK ===
... tpm2.ResponseHeader field ResponseCode
=== END STACK ===
`)
}

func (s *commandSuite) TestUnmarshalResponsePacketInvalidSize(c *C) {
	p := ResponsePacket(internal_testutil.DecodeHexString(c, "80010000001000000000"))
	_, _, _, err := p.Unmarshal(nil)
	c.Check(err, ErrorMatches, "invalid responseSize value \\(got 16, packet length 10\\)")
}

func (s *commandSuite) TestUnmarshalResponsePacketUnexpectedTPM1(c *C) {
	p := ResponsePacket(internal_testutil.DecodeHexString(c, "00c40000000a00000000"))
	_, _, _, err := p.Unmarshal(nil)
	c.Check(err, ErrorMatches, "unexpected TPM1.2 response code 0x00000000")
}

func (s *commandSuite) TestUnmarshalResponsePacketUnsuccessfulWithSessions(c *C) {
	p := ResponsePacket(internal_testutil.DecodeHexString(c, "80020000000a0000088e"))
	_, _, _, err := p.Unmarshal(nil)
	c.Check(err, ErrorMatches, "unexpcted response code 0x0000088e for TPM_ST_SESSIONS response")
}

func (s *commandSuite) TestUnmarshalResponsePacketTPM12(c *C) {
	p := ResponsePacket(internal_testutil.DecodeHexString(c, "00c40000000a0000001e"))
	rc, params, authArea, err := p.Unmarshal(nil)
	c.Check(err, IsNil)
	c.Check(params, HasLen, 0)
	c.Check(authArea, HasLen, 0)
	c.Check(rc, Equals, ResponseBadTag)
}

func (s *commandSuite) TestUnmarshalResponsePacketNoSessions(c *C) {
	p := ResponsePacket(internal_testutil.DecodeHexString(c, "80010000002c0000000000200000000000000000000000000000000000000000000000000000000000000000"))
	rc, params, authArea, err := p.Unmarshal(nil)
	c.Check(err, IsNil)
	c.Check(params, DeepEquals, internal_testutil.DecodeHexString(c, "00200000000000000000000000000000000000000000000000000000000000000000"))
	c.Check(authArea, HasLen, 0)
	c.Check(rc, Equals, ResponseSuccess)
}

func (s *commandSuite) TestUnmarshalResponsePacketWithSessions(c *C) {
	p := ResponsePacket(internal_testutil.DecodeHexString(c, "80020000001a00000000000000070005a5a5a5a5a50000010000"))
	rc, params, authArea, err := p.Unmarshal(nil)
	c.Check(err, IsNil)
	c.Check(params, DeepEquals, internal_testutil.DecodeHexString(c, "0005a5a5a5a5a5"))
	c.Check(authArea, DeepEquals, []AuthResponse{{Nonce: Nonce{}, SessionAttributes: AttrContinueSession, HMAC: Auth{}}})
	c.Check(rc, Equals, ResponseSuccess)
}

func (s *commandSuite) TestUnmarshalResponsePacketWithHandle(c *C) {
	p := ResponsePacket(internal_testutil.DecodeHexString(c, "80010000000e0000000080000002"))

	var handle Handle
	rc, params, authArea, err := p.Unmarshal(&handle)
	c.Check(err, IsNil)
	c.Check(params, HasLen, 0)
	c.Check(authArea, HasLen, 0)
	c.Check(rc, Equals, ResponseSuccess)
	c.Check(handle, Equals, Handle(0x80000002))
}

func (s *commandSuite) TestUnmarshalResponsePacketInvalidParamSize(c *C) {
	p := ResponsePacket(internal_testutil.DecodeHexString(c, "80020000001a00000000000010070005a5a5a5a5a50000010000"))
	_, _, _, err := p.Unmarshal(nil)
	c.Check(err, ErrorMatches, "cannot read parameters: unexpected EOF")
}

func (s *commandSuite) TestUnmarshalResponsePacketTooManySessions(c *C) {
	p := ResponsePacket(internal_testutil.DecodeHexString(c, "80020000002900000000000000070005a5a5a5a5a50000010000000001000000000100000000010000"))
	_, _, _, err := p.Unmarshal(nil)
	c.Check(err, ErrorMatches, "5 trailing byte\\(s\\)")
}

func (s *commandSuite) TestUseHandleContext(c *C) {
	session := &mockSessionContext{handle: 0x02000000}
	h := UseHandleContext(session)
	c.Check(h, NotNil)
	c.Check(h.Handle(), Equals, session)
	c.Check(h.Session(), IsNil)
}

func (s *commandSuite) TestUseHandleContextNil(c *C) {
	h := UseHandleContext(nil)
	c.Check(h, NotNil)
	c.Check(h.Handle(), DeepEquals, NullResource())
	c.Check(h.Session(), IsNil)
}

func (s *commandSuite) TestUseResouceContextWithAuth(c *C) {
	resource := &mockResourceContext{handle: 0x80000000}
	session := &mockSessionContext{handle: 0x02000001}
	h := UseResourceContextWithAuth(resource, session)
	c.Check(h, NotNil)
	c.Check(h.Handle(), Equals, resource)
	c.Check(h.Session(), Equals, session)
}

func (s *commandSuite) TestUseResouceContextWithAuthPW(c *C) {
	resource := &mockResourceContext{handle: 0x80000000}
	h := UseResourceContextWithAuth(resource, nil)
	c.Check(h, NotNil)
	c.Check(h.Handle(), Equals, resource)
	c.Check(h.Session(), DeepEquals, PwSession())
}

func (s *commandSuite) TestUseResouceContextWithAuthNil(c *C) {
	h := UseResourceContextWithAuth(nil, nil)
	c.Check(h, NotNil)
	c.Check(h.Handle(), DeepEquals, NullResource())
	c.Check(h.Session(), DeepEquals, PwSession())
}

func (s *commandSuite) TestCommandContextAddHandles(c *C) {
	handles := []*CommandHandleContext{
		UseResourceContextWithAuth(new(mockResourceContext), nil),
		UseResourceContextWithAuth(new(mockResourceContext), new(mockSessionContext)),
		UseHandleContext(new(mockSessionContext))}

	context := NewMockCommandContext(nil, nil)
	c.Check(context.AddHandles(handles...), Equals, context)
	c.Check(context.Cmd().Handles, HasLen, len(handles))
	c.Check(context.Cmd().Handles[0], Equals, handles[0])
	c.Check(context.Cmd().Handles[1], Equals, handles[1])
	c.Check(context.Cmd().Handles[2], Equals, handles[2])
}

func (s *commandSuite) TestCommandContextAddParams(c *C) {
	params := []interface{}{[]byte("1234"), uint16(25), OpNeq}

	context := NewMockCommandContext(nil, nil)
	c.Check(context.AddParams(params...), Equals, context)
	c.Check(context.Cmd().Params, DeepEquals, params)
}

func (s *commandSuite) TestCommandContextAddExtraSessions(c *C) {
	sessions := []SessionContext{
		new(mockSessionContext),
		new(mockSessionContext)}

	context := NewMockCommandContext(nil, nil)
	c.Check(context.AddExtraSessions(sessions...), Equals, context)
	c.Check(context.Cmd().ExtraSessions[0], Equals, sessions[0])
	c.Check(context.Cmd().ExtraSessions[1], Equals, sessions[1])
}

type mockCommandDispatcher struct {
	cmd *CmdContext

	runRsp    *RspContext
	runErr    error
	rspHandle Handle

	rsp *RspContext

	completeErr error
	rspParams   []interface{}
}

func (d *mockCommandDispatcher) RunCommand(c *CmdContext, responseHandle *Handle) (*RspContext, error) {
	d.cmd = c
	if d.runErr != nil {
		return nil, d.runErr
	}
	if responseHandle != nil {
		*responseHandle = d.rspHandle
	}
	return d.runRsp, nil
}

func (d *mockCommandDispatcher) CompleteResponse(r *RspContext, responseParams ...interface{}) error {
	d.rsp = r
	if d.completeErr != nil {
		return d.completeErr
	}
	for i := range responseParams {
		mu.MustCopyValue(responseParams[i], d.rspParams[i])
	}
	return nil
}

func (s *commandSuite) TestCommandContextRunWithoutProcessingResponseNilHandle(c *C) {
	dispatcher := &mockCommandDispatcher{runRsp: new(RspContext)}
	context := NewMockCommandContext(dispatcher, nil)

	rsp, err := context.RunWithoutProcessingResponse(nil)
	c.Check(err, IsNil)
	c.Check(dispatcher.cmd, Equals, context.Cmd())
	c.Check(rsp.Rsp(), Equals, dispatcher.runRsp)
	c.Check(rsp.Dispatcher(), Equals, dispatcher)
}

func (s *commandSuite) TestCommandContextRunWithoutProcessingResponse(c *C) {
	dispatcher := &mockCommandDispatcher{runRsp: new(RspContext), rspHandle: 0x80000002}
	context := NewMockCommandContext(dispatcher, nil)

	var handle Handle
	rsp, err := context.RunWithoutProcessingResponse(&handle)
	c.Check(err, IsNil)
	c.Check(dispatcher.cmd, Equals, context.Cmd())
	c.Check(rsp.Rsp(), Equals, dispatcher.runRsp)
	c.Check(rsp.Dispatcher(), Equals, dispatcher)
	c.Check(handle, Equals, dispatcher.rspHandle)
}

func (s *commandSuite) TestCommandContextRunWithoutProcessingResponseError(c *C) {
	dispatcher := &mockCommandDispatcher{runErr: errors.New("some error")}
	context := NewMockCommandContext(dispatcher, nil)

	rsp, err := context.RunWithoutProcessingResponse(nil)
	c.Check(rsp, IsNil)
	c.Check(err, Equals, dispatcher.runErr)
}

func (s *commandSuite) TestResponseContextComplete(c *C) {
	dispatcher := &mockCommandDispatcher{rspParams: []interface{}{[]byte("1234")}}
	context := NewMockResponseContext(dispatcher, new(RspContext))

	var data []byte
	c.Check(context.Complete(&data), IsNil)
	c.Check(dispatcher.rsp, Equals, context.Rsp())
	c.Check(data, DeepEquals, []byte("1234"))
}

func (s *commandSuite) TestResponseContextCompleteError(c *C) {
	dispatcher := &mockCommandDispatcher{completeErr: errors.New("some error")}
	context := NewMockResponseContext(dispatcher, new(RspContext))

	c.Check(context.Complete(), Equals, dispatcher.completeErr)
}

func (s *commandSuite) TestCommandContextRunWithNilHandle(c *C) {
	dispatcher := &mockCommandDispatcher{runRsp: new(RspContext), rspParams: []interface{}{[]byte("1234")}}
	context := NewMockCommandContext(dispatcher, nil)

	var data []byte
	c.Check(context.Run(nil, &data), IsNil)
	c.Check(dispatcher.cmd, Equals, context.Cmd())
	c.Check(dispatcher.rsp, Equals, dispatcher.runRsp)
	c.Check(data, DeepEquals, []byte("1234"))
}

func (s *commandSuite) TestCommandContextRun(c *C) {
	dispatcher := &mockCommandDispatcher{runRsp: new(RspContext), rspHandle: 0x80000003}
	context := NewMockCommandContext(dispatcher, nil)

	var handle Handle
	c.Check(context.Run(&handle), IsNil)
	c.Check(dispatcher.cmd, Equals, context.Cmd())
	c.Check(dispatcher.rsp, Equals, dispatcher.runRsp)
	c.Check(handle, Equals, dispatcher.rspHandle)
}

func (s *commandSuite) TestCommandContextRunWithError(c *C) {
	dispatcher := &mockCommandDispatcher{runErr: errors.New("some error")}
	context := NewMockCommandContext(dispatcher, nil)

	c.Check(context.Run(nil), Equals, dispatcher.runErr)
}

func (s *commandSuite) TestCommandContextRunWithCompleteError(c *C) {
	dispatcher := &mockCommandDispatcher{runRsp: new(RspContext), completeErr: errors.New("some error")}
	context := NewMockCommandContext(dispatcher, nil)

	c.Check(context.Run(nil), Equals, dispatcher.completeErr)
}
