// Copyright 2019 DxChain, All rights reserved.
// Use of this source code is governed by an Apache
// License 2.0 that can be found in the LICENSE file.

package storagehostmanager

import (
	"testing"

	"github.com/DxChainNetwork/godx/storage"
)

func TestInteractionName(t *testing.T) {
	tests := []struct {
		it   InteractionType
		name string
	}{
		{InteractionGetConfig, "host config scan"},
		{InteractionCreateContract, "create contract"},
		{InteractionRenewContract, "renew contract"},
		{InteractionUpload, "upload"},
		{InteractionDownload, "download"},
	}
	for index, test := range tests {
		name := InteractionTypeToName(test.it)
		if name != test.name {
			t.Errorf("test %d interaction name not expected. Got %v, Expect %v", index, name, test.name)
		}
		it := InteractionNameToType(test.name)
		if it != test.it {
			t.Errorf("test %d interaction type not expected. Got %v, Expect %v", index, it, test.it)
		}
	}
}

func TestInteractionNameInvalid(t *testing.T) {
	invalidName := "ssss"
	if InteractionNameToType(invalidName) != InteractionInvalid {
		t.Errorf("invalid name does not yield expected result")
	}
	if InteractionTypeToName(InteractionInvalid) != "" {
		t.Errorf("invalid type does not yield empty name")
	}
	if InteractionTypeToName(InteractionType(100)) != "" {
		t.Errorf("invalid type does not yield empty name")
	}
}

func TestInteractionWeight(t *testing.T) {
	tests := []struct {
		it             InteractionType
		expectedWeight float64
	}{
		{InteractionInvalid, 0},
		{InteractionGetConfig, 1},
		{InteractionCreateContract, 2},
		{InteractionRenewContract, 2},
		{InteractionUpload, 5},
		{InteractionDownload, 10},
	}
	for _, test := range tests {
		res := interactionWeight(test.it)
		if res != test.expectedWeight {
			t.Errorf("test %v weight not expected", test.it)
		}
	}
}

func TestInteractionInitiate(t *testing.T) {
	tests := []struct {
		successBefore   float64
		failedBefore    float64
		expectedSuccess float64
		expectedFailed  float64
	}{
		{1, 0, 1, 0},
		{0, 1, 0, 1},
		{0, 0, initialSuccessfulInteractionFactor, initialFailedInteractionFactor},
	}
	for _, test := range tests {
		info := storage.HostInfo{
			SuccessfulInteractionFactor: test.successBefore,
			FailedInteractionFactor:     test.failedBefore,
		}
		interactionInitiate(&info)
		if info.SuccessfulInteractionFactor != test.expectedSuccess {
			t.Errorf("successful interaction not expected")
		}
		if info.FailedInteractionFactor != test.expectedFailed {
			t.Errorf("failed interaction not expected")
		}
	}
}