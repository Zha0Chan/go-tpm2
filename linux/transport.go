// Copyright 2019 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package linux

import (
	"io"
	"os"

	"github.com/canonical/go-tpm2/internal/transportutil"
)

const (
	maxCommandSize  = 4096
	maxResponseSize = 4096
)

type fileStatter interface {
	Stat() (os.FileInfo, error)
}

// Tcti represents a connection to a Linux TPM character device.
//
// Deprecated: Use [Transport].
type Tcti = Transport

// Transport represents a connection to a Linux TPM character device.
type Transport struct {
	r       io.Reader
	w       io.Writer
	closer  io.Closer
	statter fileStatter
}

func newTransport(file *tpmFile) *Transport {
	return &Transport{
		r:       transportutil.BufferResponses(file, maxResponseSize),
		w:       transportutil.BufferCommands(file, maxCommandSize),
		closer:  file,
		statter: file,
	}
}

// Read implmements [tpm2.Transport].
func (d *Transport) Read(data []byte) (int, error) {
	return d.r.Read(data)
}

// Write implmements [tpm2.Transport].
func (d *Transport) Write(data []byte) (int, error) {
	return d.w.Write(data)
}

// Close implements [tpm2.Transport.Close].
func (d *Transport) Close() error {
	return d.closer.Close()
}
