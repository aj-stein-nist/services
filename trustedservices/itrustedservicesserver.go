// Copyright 2022-2023 Contributors to the Veraison project.
// SPDX-License-Identifier: Apache-2.0
package trustedservices

import (
	"github.com/spf13/viper"
	"github.com/veraison/services/handler"
	"github.com/veraison/services/plugin"
	"github.com/veraison/services/proto"
)

type ITrustedServicesServer interface {
	Init(cfg *viper.Viper, pm plugin.IManager[handler.IEvidenceHandler]) error
	Close() error
	Run() error

	proto.TrustedServicesServer
}
