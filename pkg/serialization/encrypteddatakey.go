// Copyright Chainify Group LTD. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package serialization

import (
	"bytes"
	"fmt"
	"math"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"

	"github.com/chainifynet/aws-encryption-sdk-go/pkg/keys"
	"github.com/chainifynet/aws-encryption-sdk-go/pkg/logger"
	"github.com/chainifynet/aws-encryption-sdk-go/pkg/utils/conv"
)

var (
	log = logger.L().With().Logger().Level(zerolog.DebugLevel)
)

type providerIdentity string

const (
	awsKmsProviderID providerIdentity = "aws-kms"
)

var EDK = edk{
	ProviderID: awsKmsProviderID,
	LenFields:  3,
}

type edk struct {
	ProviderID providerIdentity
	LenFields  int
}

var (
	MaxEncryptedDataKeysError = errors.New("maximum number of encrypted data keys")      // TODO andrew change to unexported
	MinEncryptedDataKeysError = errors.New("minimum number of encrypted data keys is 1") // TODO andrew change to unexported
	IncompleteBufferErr       = errors.New("incomplete buffer")                          // TODO andrew change to unexported
)

type encryptedDataKey struct {
	providerIDLen       int              // 2, lenFieldBytes, providerIDLen is length of ProviderID, always present.
	ProviderID          providerIdentity // string bytes, only awsKmsProviderID supported as ProviderID, always present.
	providerInfoLen     int              // 2, lenFieldBytes, providerInfoLen is length of ProviderInfo, always present.
	ProviderInfo        string           // string bytes, ProviderInfo usually is KMS Key ID ARN, not an alias!
	encryptedDataKeyLen int              // 2, lenFieldBytes, encryptedDataKeyLen is length of encryptedDataKey
	encryptedDataKey    []byte           // bytes, encryptedDataKey is encrypted data key content bytes
}

func (e edk) new(providerID providerIdentity, providerInfo string, encryptedDataKeyData []byte) (*encryptedDataKey, error) {

	if providerID != EDK.ProviderID {
		return nil, fmt.Errorf("providerIdentity is not supported. Use providerIdentity.awsKmsProviderID")
	}

	if len(providerInfo) > math.MaxUint32 {
		return nil, fmt.Errorf("ProviderInfo is too large, out of range MaxUint32")
	}

	return &encryptedDataKey{
		providerIDLen:       len(providerID),
		ProviderID:          providerID,
		providerInfoLen:     len(providerInfo),
		ProviderInfo:        providerInfo,
		encryptedDataKeyLen: len(encryptedDataKeyData),
		encryptedDataKey:    encryptedDataKeyData,
	}, nil
}

func (edk encryptedDataKey) String() string {
	return fmt.Sprintf("%#v", edk)
}

func (edk encryptedDataKey) asKey() keys.EncryptedDataKeyI {
	return keys.NewEncryptedDataKey(
		keys.WithKeyMeta(string(edk.ProviderID), edk.ProviderInfo),
		edk.encryptedDataKey,
	)
}

func (edk encryptedDataKey) len() int {
	return (EDK.LenFields * lenFieldBytes) +
		edk.providerIDLen +
		edk.providerInfoLen +
		edk.encryptedDataKeyLen
}

func (edk encryptedDataKey) bytes() []byte {
	var buf []byte
	buf = make([]byte, 0, edk.len())
	buf = append(buf, conv.FromInt.Uint16BigEndian(edk.providerIDLen)...)
	buf = append(buf, []byte(edk.ProviderID)...)
	buf = append(buf, conv.FromInt.Uint16BigEndian(edk.providerInfoLen)...)
	buf = append(buf, []byte(edk.ProviderInfo)...)
	buf = append(buf, conv.FromInt.Uint16BigEndian(edk.encryptedDataKeyLen)...)
	buf = append(buf, edk.encryptedDataKey...)
	return buf
}

func (e edk) AsKeys(msgEDKs []encryptedDataKey) []keys.EncryptedDataKeyI {
	var edks []keys.EncryptedDataKeyI
	edks = make([]keys.EncryptedDataKeyI, 0, len(msgEDKs))
	for _, k := range msgEDKs {
		edks = append(edks, k.asKey())
	}
	return edks
}

func (e edk) validateMinMaxEDKs(keys int, max int) error {
	if keys <= 0 {
		return errors.Wrap(MinEncryptedDataKeysError, "reached limit")
	}
	if keys > max {
		return errors.Wrap(MaxEncryptedDataKeysError, "reached max limit")
	}
	return nil
}

func (e edk) fromBufferWithCount(buf *bytes.Buffer) (int, []encryptedDataKey, error) {
	if buf.Len() < countFieldBytes {
		return 0, nil, errors.Wrap(IncompleteBufferErr, "deserialize encrypted data keys count")
	}

	encryptedDataKeyCount := fieldReader.ReadCountField(buf)
	if encryptedDataKeyCount <= 0 {
		return 0, nil, fmt.Errorf("encrypted data keys not found in message header")
	}

	var edks []encryptedDataKey
	for i := 0; i < encryptedDataKeyCount; i++ {
		encDataKey, err := e.fromBuffer(buf)
		if err != nil {
			return 0, nil, errors.Wrap(err, "cant deserialize expected encrypted data key")
		}
		edks = append(edks, *encDataKey)
	}

	return encryptedDataKeyCount, edks, nil
}

func (e edk) FromEDKs(list []keys.EncryptedDataKeyI) ([]encryptedDataKey, error) {
	var edks []encryptedDataKey
	for _, keyI := range list {
		encDataKey, err := e.new(providerIdentity(keyI.KeyProvider().ProviderID), keyI.KeyProvider().KeyID, keyI.EncryptedDataKey())
		if err != nil {
			return nil, err
		}
		edks = append(edks, *encDataKey)
	}
	return edks, nil
}

func (e edk) fromBuffer(buf *bytes.Buffer) (*encryptedDataKey, error) {
	if buf.Len() < (lenFieldBytes * e.LenFields) {
		return nil, errors.Wrap(IncompleteBufferErr, "deserialize encrypted data key")
	}
	providerIDLen, err := fieldReader.ReadLenField(buf)
	if err != nil {
		return nil, fmt.Errorf("cant deserialize encrypted data key providerIDLen, %w", err)
	}
	providerID := buf.Next(providerIDLen)

	providerInfoLen, err := fieldReader.ReadLenField(buf)
	if err != nil {
		return nil, fmt.Errorf("cant deserialize encrypted data key providerInfoLen, %w", err)
	}
	providerInfo := buf.Next(providerInfoLen)

	encryptedDataKeyLen, err := fieldReader.ReadLenField(buf)
	if err != nil {
		return nil, fmt.Errorf("cant deserialize encrypted data key encryptedDataKeyLen, %w", err)
	}
	encryptedDataKeyData := buf.Next(encryptedDataKeyLen)

	log.Trace().
		Int("len", providerIDLen).
		Str("bytes", logger.FmtBytes(providerID)).
		Str("text", string(providerID)).
		Msg("providerID")

	log.Trace().
		Int("len", providerInfoLen).
		Str("byte", logger.FmtBytes(providerInfo)).
		Str("text", string(providerInfo)).
		Msg("providerInfo")

	log.Trace().
		Int("len", encryptedDataKeyLen).
		Msg("encryptedDataKeyData")

	//log.Trace().MsgFunc(logger.FmtHex("encryptedDataKeyData", encryptedDataKeyData))
	//log.Trace().Str("encryptedDataKeyDataBytes", fmt.Sprintf("%#v", encryptedDataKeyData)).Msg("encryptedDataKeyDataBytes")

	return e.new(providerIdentity(providerID), string(providerInfo), encryptedDataKeyData)
}