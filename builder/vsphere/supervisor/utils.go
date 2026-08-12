// © Broadcom. All Rights Reserved.
// The term "Broadcom" refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: MPL-2.0

package supervisor

import (
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

type PackerLogger struct {
	UI packersdk.Ui
}

func (pl *PackerLogger) Info(msg string, args ...any) {
	pl.UI.Message(fmt.Sprintf(msg, args...))
}

func (pl *PackerLogger) Error(msg string, args ...any) {
	pl.UI.Errorf(msg, args...)
}

func CheckRequiredStates(state multistep.StateBag, keys ...string) error {
	for _, key := range keys {
		if _, ok := state.GetOk(key); !ok {
			return fmt.Errorf("missing required state: %s", key)
		}
	}

	return nil
}
