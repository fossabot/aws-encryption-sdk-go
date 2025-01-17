// Copyright Chainify Group LTD. or its affiliates. All Rights Reserved.
// SPDX-License-Identifier: Apache-2.0

package crypto

import (
	"bytes"
	"context"
	"crypto/hmac"
	"fmt"

	"github.com/chainifynet/aws-encryption-sdk-go/pkg/crypto/signature"
	"github.com/chainifynet/aws-encryption-sdk-go/pkg/helpers/bodyaad"
	"github.com/chainifynet/aws-encryption-sdk-go/pkg/helpers/policy"
	"github.com/chainifynet/aws-encryption-sdk-go/pkg/model"
	"github.com/chainifynet/aws-encryption-sdk-go/pkg/serialization"
	"github.com/chainifynet/aws-encryption-sdk-go/pkg/suite"
	"github.com/chainifynet/aws-encryption-sdk-go/pkg/utils/keyderivation"
)

// decrypt ciphertext decryption
func (d *decrypter) decrypt(ctx context.Context, ciphertext []byte) ([]byte, *serialization.MessageHeader, error) {
	var b []byte
	b = make([]byte, len(ciphertext))
	copy(b, ciphertext)
	if len(ciphertext) == 0 || len(b) == 0 {
		return nil, nil, fmt.Errorf("empty ciphertext")
	}

	// early stage check if cipher text contains needed first byte of message version
	// by doing this we avoid mistakes with base64 byte sequence
	if ciphertext[0] != firstByteEncryptedMessage {
		return nil, nil, fmt.Errorf("first byte does not contain message version: %w", ErrInvalidMessage)
	}
	buf := bytes.NewBuffer(b)

	if err := d.decryptHeader(ctx, buf); err != nil {
		return nil, nil, err
	}

	body, err := d.decryptBody(buf)
	if err != nil {
		return nil, nil, err
	}

	if d.verifier != nil {
		footer, errFooter := serialization.MessageFooter.FromBuffer(d.header.AlgorithmSuite, buf)
		if errFooter != nil {
			return nil, nil, errFooter
		}

		if errSig := d.verifier.Verify(footer.Signature); errSig != nil {
			return nil, nil, errSig
		}
	}
	// TODO check if alg is non-signing, but footer has signature, return error

	return body, d.header, nil
}

func (d *decrypter) decryptHeader(ctx context.Context, buf *bytes.Buffer) error {
	header, headerAuth, err := serialization.DeserializeHeader(buf, d.config.MaxEncryptedDataKeys())
	if err != nil {
		return err
	}

	if errPolicy := policy.Commitment.ValidatePolicyOnDecrypt(d.config.CommitmentPolicy(), header.AlgorithmSuite); errPolicy != nil {
		return errPolicy
	}

	if header.AlgorithmSuite.IsSigning() {
		d.verifier = signature.NewECCVerifier(
			header.AlgorithmSuite.Authentication.HashFunc,
			header.AlgorithmSuite.Authentication.Algorithm,
		)
		if err := d.updateVerifier(header.Bytes()); err != nil {
			return err
		}
		if err := d.updateVerifier(headerAuth.Serialize()); err != nil {
			return err
		}
	}

	dmr := model.DecryptionMaterialsRequest{
		Algorithm:         header.AlgorithmSuite,
		EncryptedDataKeys: serialization.EDK.AsKeys(header.EncryptedDataKeys),
		EncryptionContext: header.AADData.AsEncryptionContext(),
	}

	decMaterials, err := d.cmm.DecryptMaterials(ctx, dmr)
	if err != nil {
		return fmt.Errorf("decrypt materials: %w", err)
	}

	if d.verifier != nil {
		if errLK := d.verifier.LoadECCKey(decMaterials.VerificationKey()); errLK != nil {
			return fmt.Errorf("decrypt verifier error: %w", errLK)
		}
	}

	derivedDataKey, err := keyderivation.DeriveDataEncryptionKey(decMaterials.DataKey().DataKey(), header.AlgorithmSuite, header.MessageID)
	if err != nil {
		return fmt.Errorf("decrypt key derivation error: %w", err)
	}

	if header.AlgorithmSuite.IsCommitting() {
		expectedCommitmentKey, err := keyderivation.CalculateCommitmentKey(decMaterials.DataKey().DataKey(), header.AlgorithmSuite, header.MessageID)
		if err != nil {
			return fmt.Errorf("decrypt calculate commitment key error: %w", err)
		}

		if ok := hmac.Equal(expectedCommitmentKey, header.AlgorithmSuiteData); !ok {
			return fmt.Errorf("key commitment validation failed: key identity does not match the identity asserted in the message")
		}
	}

	if errHeaderAuth := d.aeadDecrypter.ValidateHeaderAuth(derivedDataKey, headerAuth.AuthData(), header.Bytes()); errHeaderAuth != nil {
		return fmt.Errorf("decrypt header auth error: %w", errHeaderAuth)
	}

	if d._derivedDataKey != nil {
		return fmt.Errorf("decrypt derived data key already exists")
	}
	d._derivedDataKey = derivedDataKey

	if d.header != nil {
		return fmt.Errorf("decrypt header already exists")
	}
	d.header = header

	return nil
}

func (d *decrypter) decryptBody(buf *bytes.Buffer) ([]byte, error) {
	body, err := serialization.DeserializeBody(buf, d.header.AlgorithmSuite, d.header.FrameLength)
	if err != nil {
		return nil, fmt.Errorf("body error: %w", err)
	}

	plaintext := new(bytes.Buffer)
	readBytes := 0

	for _, frame := range body.Frames() {
		associatedData := bodyaad.BodyAAD.ContentAADBytes(
			d.header.MessageID,
			bodyaad.BodyAAD.ContentString(suite.FramedContent, frame.IsFinal()),
			frame.SequenceNumber(),
			len(frame.EncryptedContent()),
		)
		b, errAead := d.aeadDecrypter.Decrypt(
			d._derivedDataKey,
			frame.IV(),
			frame.EncryptedContent(),
			frame.AuthenticationTag(),
			associatedData,
		)
		if errAead != nil {
			return nil, fmt.Errorf("decrypt frame error: %w", errAead)
		}
		readBytes += len(b)
		plaintext.Write(b)
		// if alg is signing, write each frame bytes to verifier to update message hash
		if d.verifier != nil {
			if err := d.updateVerifier(frame.Bytes()); err != nil {
				return nil, err
			}
		}
	}

	if plaintext.Len() != readBytes {
		return nil, fmt.Errorf("malformed body message size")
	}

	var plaintextData []byte
	plaintextData = make([]byte, plaintext.Len())

	wb, err := plaintext.Read(plaintextData)
	if err != nil {
		return nil, fmt.Errorf("malformed body message size: %w", err)
	}
	if wb != readBytes {
		return nil, fmt.Errorf("malformed body message size")
	}
	plaintext.Reset()

	return plaintextData, nil
}

func (d *decrypter) updateVerifier(b []byte) error {
	if _, err := d.verifier.Write(b); err != nil {
		return fmt.Errorf("verifier write error: %w", err)
	}
	return nil
}
