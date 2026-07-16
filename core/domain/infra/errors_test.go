package infra

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrorTypes(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{
			name: "resource not found",
			err:  &ErrResourceNotFound{ResourceID: "my-bucket"},
			msg:  `resource "my-bucket" not found`,
		},
		{
			name: "provider not found",
			err:  &ErrProviderNotFound{ProviderID: "aws"},
			msg:  `provider "aws" not found — is it registered?`,
		},
		{
			name: "provider already registered",
			err:  &ErrProviderAlreadyRegistered{ProviderID: "aws"},
			msg:  `provider "aws" is already registered`,
		},
		{
			name: "validation error",
			err:  &ErrValidation{ResourceID: "r1", Message: "invalid name"},
			msg:  `resource "r1" validation failed: invalid name`,
		},
		{
			name: "cycle detected",
			err:  &ErrCycleDetected{StackName: "prod"},
			msg:  `dependency cycle detected in stack "prod"`,
		},
		{
			name: "state serial mismatch",
			err:  &ErrStateSerialMismatch{Expected: 5, Actual: 3},
			msg:  "state serial mismatch: expected 5, got 3 — state was modified concurrently",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.msg, tc.err.Error())
		})
	}
}

func TestErrorWrapping(t *testing.T) {
	inner := errors.New("inner error")

	lockErr := &ErrLockAcquisition{Info: LockInfo{ID: "lock1", Operation: "apply"}, Err: inner}
	assert.ErrorIs(t, lockErr, inner)

	provErr := &ErrProviderFailed{ProviderID: "aws", ResourceID: "r1", Operation: "create", Err: inner}
	assert.ErrorIs(t, provErr, inner)
}
