// Copyright 2022-2023 Contributors to the Veraison project.
// SPDX-License-Identifier: Apache-2.0
package vts

import (
	"github.com/veraison/services/proto"
)

type IVTSClient interface {
	proto.TrustedServicesClient
}
