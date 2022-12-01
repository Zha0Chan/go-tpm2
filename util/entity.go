// Copyright 2022 Canonical Ltd.
// Licensed under the LGPLv3 with static-linking exception.
// See LICENCE file for details.

package util

import (
	"github.com/canonical/go-tpm2"
)

// Entity is a type that has a name.
type Entity interface {
	Name() tpm2.Name
}
