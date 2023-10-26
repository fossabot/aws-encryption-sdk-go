// Copyright Chainify Group LTD. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/chainifynet/aws-encryption-sdk-go/pkg/keys"
	"github.com/chainifynet/aws-encryption-sdk-go/pkg/suite"
)

type dataKeyMock struct {
	dk []byte
	keys.DataKeyI
}

func (d *dataKeyMock) DataKey() []byte {
	return d.dk
}

func Test_deriveDataEncryptionKey(t *testing.T) {
	tests := []struct {
		name      string
		dk        keys.DataKeyI
		alg       *suite.AlgorithmSuite
		messageID []byte
		exp       []byte
	}{
		{"key1", &dataKeyMock{dk: []byte{0x01}}, suite.AES_256_GCM_HKDF_SHA512_COMMIT_KEY, []byte{}, []byte{0xc8, 0xdb, 0xb9, 0x26, 0xc7, 0xa4, 0xf9, 0xc9, 0x60, 0x6, 0x90, 0x34, 0x2d, 0xf6, 0x74, 0xd7, 0xf9, 0xb9, 0xb8, 0x20, 0x70, 0x5f, 0xe3, 0xfc, 0x84, 0x4b, 0x8f, 0x71, 0x8b, 0xca, 0x5, 0x2a}},
		{"key2", &dataKeyMock{dk: []byte{0x01}}, suite.AES_256_GCM_HKDF_SHA512_COMMIT_KEY, []byte{0x01}, []byte{0xd4, 0xfd, 0xcf, 0x9, 0x81, 0xa, 0x67, 0x64, 0xdd, 0xe7, 0x4d, 0x52, 0x42, 0xdf, 0x1c, 0x23, 0xfa, 0x3, 0x41, 0xaa, 0x7b, 0x58, 0x23, 0xf0, 0xf1, 0x69, 0xdc, 0x39, 0x36, 0xd9, 0x0, 0x78}},
		{"key3", &dataKeyMock{dk: []byte{0x02}}, suite.AES_256_GCM_HKDF_SHA512_COMMIT_KEY, []byte{}, []byte{0x5d, 0x2, 0x70, 0x41, 0x30, 0x42, 0x1e, 0xee, 0x1d, 0x4, 0xae, 0x6a, 0xdb, 0x1, 0x9d, 0x8, 0x67, 0xea, 0x77, 0x5b, 0x3e, 0x2f, 0xdc, 0xb4, 0xfe, 0x31, 0x16, 0xbf, 0xa9, 0xa6, 0x3d, 0x79}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			derivedKey, err := deriveDataEncryptionKey(tt.dk, tt.alg, tt.messageID)
			assert.NoError(t, err)
			assert.Equal(t, tt.exp, derivedKey)
			assert.Len(t, derivedKey, tt.alg.EncryptionSuite.DataKeyLen)
		})
	}
}