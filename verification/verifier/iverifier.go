// Copyright 2022-2023 Contributors to the Veraison project.
// SPDX-License-Identifier: Apache-2.0
package verifier

import (
	"github.com/veraison/services/proto"
)

type IVerifier interface {
	GetVTSState() (*proto.ServiceState, error)
	GetPublicKey() (*proto.PublicKey, error)
	IsSupportedMediaType(mt string) (bool, error)
	SupportedMediaTypes() ([]string, error)
	ProcessEvidence(
		tenantID string, nonce []byte, data []byte, mt string, teeReport bool,
	) ([]byte, error)
}
