// Copyright Chainify Group LTD. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package keys

import (
	"context"
	"errors"

	"github.com/chainifynet/aws-encryption-sdk-go/pkg/suite"
)

var (
	ErrDecryptKey      = errors.New("unable to decrypt data key")
	ErrGenerateDataKey = errors.New("unable to generate data key")
	ErrEncryptKey      = errors.New("unable to encrypt data key")
)

type KeyMeta struct {
	ProviderID string
	KeyID      string
}

func WithKeyMeta(providerID, keyID string) KeyMeta {
	return KeyMeta{
		ProviderID: providerID,
		KeyID:      keyID,
	}
}

func (km KeyMeta) Equal(other KeyMeta) bool {
	if km.ProviderID != other.ProviderID || km.KeyID != other.KeyID {
		return false
	}
	return true
}

func (km KeyMeta) String() string {
	return km.ProviderID + "__" + km.KeyID
}

type MasterKeyBase interface {
	KeyID() string
	Metadata() KeyMeta
	OwnsDataKey(key Key) bool
	GenerateDataKey(ctx context.Context, alg *suite.AlgorithmSuite, ec suite.EncryptionContext) (DataKeyI, error)
	EncryptDataKey(ctx context.Context, dataKey DataKeyI, alg *suite.AlgorithmSuite, ec suite.EncryptionContext) (EncryptedDataKeyI, error)
	DecryptDataKey(ctx context.Context, encryptedDataKey EncryptedDataKeyI, alg *suite.AlgorithmSuite, ec suite.EncryptionContext) (DataKeyI, error)
}

type KeyEntry[V any] struct {
	Entry V
}

func (ke KeyEntry[V]) GetEntry() V {
	return ke.Entry
}

func NewKeyEntry[V MasterKeyBase](key V) KeyEntry[V] {
	newEntry := KeyEntry[V]{Entry: key}
	return newEntry
}

func NewKeyEntryPtr[V MasterKeyBase](key V) *KeyEntry[V] {
	newEntry := new(KeyEntry[V])
	newEntry.Entry = key
	return newEntry
}

// 				Basic keys

type KeyBase interface {
	KeyProvider() KeyMeta
	KeyID() string
}

type Key interface {
	KeyBase
}

type DataKeyI interface {
	Key

	EncryptedDataKey() []byte
	DataKey() []byte
}

type EncryptedDataKeyI interface {
	Key
	EncryptedDataKey() []byte
}
