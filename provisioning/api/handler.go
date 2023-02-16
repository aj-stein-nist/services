// Copyright 2022-2023 Contributors to the Veraison project.
// SPDX-License-Identifier: Apache-2.0
package api

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/veraison/services/config"
	"github.com/veraison/services/handler"
	"github.com/veraison/services/plugin"
	"github.com/veraison/services/proto"
	"github.com/veraison/services/trustedservices"
	"go.uber.org/zap"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

type IHandler interface {
	Submit(c *gin.Context)
	GetServiceState(c *gin.Context)
}

type Handler struct {
	PluginManager plugin.IManager[handler.IEndorsementHandler]
	VTSClient     trustedservices.ITrustedServicesClient

	logger *zap.SugaredLogger
}

func NewHandler(
	pm plugin.IManager[handler.IEndorsementHandler],
	sc trustedservices.ITrustedServicesClient,
	logger *zap.SugaredLogger,
) IHandler {
	return &Handler{
		PluginManager: pm,
		VTSClient:     sc,
		logger:        logger,
	}
}

type ProvisioningSession struct {
	Status        string  `json:"status"`
	Expiry        string  `json:"expiry"`
	FailureReason *string `json:"failure-reason,omitempty"`
}

const (
	ProvisioningSessionMediaType = "application/vnd.veraison.provisioning-session+json"
)

func (o *Handler) GetServiceState(c *gin.Context) {
	state := proto.ServiceState{
		ServerVersion: config.Version,
	}

	vtsState, err := o.VTSClient.GetServiceState(context.TODO(), &emptypb.Empty{})
	if err != nil {
		ReportProblem(c,
			http.StatusInternalServerError,
			fmt.Sprintf("could not retrieve service state: %v", err),
		)
		return
	}

	apiMediaTypeList, err := structpb.NewList([]interface{}{ProvisioningSessionMediaType})
	if err != nil {
		panic(err) // Should never happen as the value above is hard-coded.
	}

	handlerMediaTypeList, err := proto.NewStringList(o.PluginManager.GetRegisteredMediaTypes())
	if err != nil {
		ReportProblem(c,
			http.StatusInternalServerError,
			fmt.Sprintf("could not retrieve handler media types: %v", err),
		)
		return
	}

	state.SupportedMediaTypes = map[string]*structpb.ListValue{
		"endorsement-provisioning/v1": apiMediaTypeList,
		"handler":                     handlerMediaTypeList.AsListValue(),
	}

	if vtsState.Status == proto.ServiceStatus_DOWN {
		state.Status = proto.ServiceStatus_INITIALIZING
	} else {
		state.Status = proto.ServiceStatus_READY
	}

	c.Header("Content-Type", proto.ServiceStateMediaType)
	c.JSON(http.StatusOK, &state)
}

func (o *Handler) Submit(c *gin.Context) {
	// read the accept header and make sure that it's compatible with what we
	// support
	offered := c.NegotiateFormat(ProvisioningSessionMediaType)
	if offered != ProvisioningSessionMediaType {
		ReportProblem(c,
			http.StatusNotAcceptable,
			fmt.Sprintf("the only supported output format is %s", ProvisioningSessionMediaType),
		)
		return
	}

	// read media type
	mediaType := c.Request.Header.Get("Content-Type")

	if !o.PluginManager.IsRegisteredMediaType(mediaType) {
		mediaTypes := o.PluginManager.GetRegisteredMediaTypes()
		c.Header("Accept", strings.Join(mediaTypes, ", "))
		ReportProblem(c,
			http.StatusUnsupportedMediaType,
			fmt.Sprintf("no active plugin found for %s", mediaType),
		)
		return
	}

	// read body
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		ReportProblem(c,
			http.StatusBadRequest,
			fmt.Sprintf("error reading body: %s", err),
		)
		return
	}

	if len(payload) == 0 {
		ReportProblem(c,
			http.StatusBadRequest,
			"empty body",
		)
		return
	}

	// From here onwards we assume that a provisioning session exists and that
	// every further communication (apart from panics) will be through that
	// object instead of using RFC7807 Problem Details.  We can add support for
	// stashing session state later on when we will implement the asynchronous
	// API model.  For now, the object is created opportunistically.

	// pass data to the identified plugin for normalisation
	rsp, err := o.dispatch(mediaType, payload)
	if err != nil {
		o.logger.Errorw("session failed", "error", err)

		if errors.As(err, &trustedservices.NoConnectionError{}) {
			ReportProblem(c,
				http.StatusInternalServerError,
				err.Error(),
			)
			return
		}

		sendFailedProvisioningSession(
			c,
			fmt.Sprintf("handler manager returned error: %s", err),
		)
		return
	}

	// forward normalised data to the endorsement store
	if err := o.store(rsp); err != nil {
		o.logger.Errorw("session failed", "error", err)

		if errors.As(err, &trustedservices.NoConnectionError{}) {
			ReportProblem(c,
				http.StatusInternalServerError,
				err.Error(),
			)
			return
		}

		sendFailedProvisioningSession(
			c,
			fmt.Sprintf("endorsement store returned error: %s", err),
		)
		return
	}

	sendSuccessfulProvisioningSession(c)
}

func (o *Handler) dispatch(
	mediaType string,
	payload []byte,
) (*handler.EndorsementHandlerResponse, error) {
	handlerPlugin, err := o.PluginManager.LookupByMediaType(mediaType)
	if err != nil {
		return nil, err
	}

	return handlerPlugin.Decode(payload)
}

func (o *Handler) store(rsp *handler.EndorsementHandlerResponse) error {
	for _, ta := range rsp.TrustAnchors {
		taReq := &proto.AddTrustAnchorRequest{TrustAnchor: ta}

		taRes, err := o.VTSClient.AddTrustAnchor(context.TODO(), taReq)
		if err != nil {
			return fmt.Errorf("store operation failed for trust anchor: %w", err)
		}

		if !taRes.GetStatus().Result {
			return fmt.Errorf(
				"store operation failed for trust anchor: %s",
				taRes.Status.GetErrorDetail(),
			)
		}
	}

	for _, refVal := range rsp.ReferenceValues {
		refValReq := &proto.AddRefValuesRequest{
			ReferenceValues: []*proto.Endorsement{
				refVal,
			},
		}

		refValRes, err := o.VTSClient.AddRefValues(context.TODO(), refValReq)
		if err != nil {
			return fmt.Errorf("store operation failed for reference values: %w", err)
		}

		if !refValRes.GetStatus().Result {
			return fmt.Errorf(
				"store operation failed for reference values: %s",
				refValRes.Status.GetErrorDetail(),
			)
		}
	}

	return nil
}

func sendFailedProvisioningSession(c *gin.Context, failureReason string) {
	c.Header("Content-Type", ProvisioningSessionMediaType)
	c.JSON(
		http.StatusOK,
		&ProvisioningSession{
			Status:        "failed",
			Expiry:        time.Now().Format(time.RFC3339),
			FailureReason: &failureReason,
		},
	)
}

func sendSuccessfulProvisioningSession(c *gin.Context) {
	c.Header("Content-Type", ProvisioningSessionMediaType)
	c.JSON(
		http.StatusOK,
		&ProvisioningSession{
			Status: "success",
			Expiry: time.Now().Format(time.RFC3339),
		},
	)
}
