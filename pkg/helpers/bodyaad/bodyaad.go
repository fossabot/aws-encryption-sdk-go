// Copyright Chainify Group LTD. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package bodyaad

import (
	"github.com/rs/zerolog/log"

	"github.com/chainifynet/aws-encryption-sdk-go/pkg/suite"
	"github.com/chainifynet/aws-encryption-sdk-go/pkg/utils/conv"
)

const (
	seqNumLen = int(4) // seqNum as big-endian 32-bit unsigned integer
	lengthLen = int(8) // length as big-endian 64-bit unsigned integer
)

var BodyAAD bodyAAD //nolint:gochecknoglobals

type bodyAAD struct{}

func (bodyAAD) ContentString(contentType suite.ContentType, finalFrame bool) []byte {
	if contentType != suite.FramedContent {
		// TODO refactor to return ([]byte, error), dont panic here!
		log.Info().Msgf("%v", suite.CommitmentPolicyForbidEncryptAllowDecrypt)
		log.Panic().Msg("NonFramed content type not supported")
	}
	if finalFrame {
		return []byte(suite.ContentAADFinalFrame)
	}
	return []byte(suite.ContentAADFrame)
}

func (bodyAAD) ContentAADBytes(messageID, contentString []byte, seqNum, length int) []byte {
	bufLen := len(messageID) +
		len(contentString) +
		seqNumLen + // 4, seqNum as big-endian 32-bit unsigned integer
		lengthLen // 8, length as big-endian 64-bit unsigned integer

	var buf []byte
	buf = make([]byte, 0, bufLen)
	buf = append(buf, messageID...)
	buf = append(buf, contentString...)
	buf = append(buf, conv.FromInt.Uint32BigEndian(seqNum)...)
	buf = append(buf, conv.FromInt.Uint64BigEndian(length)...)
	return buf
}
