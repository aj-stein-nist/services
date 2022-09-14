// Copyright 2022 Contributors to the Veraison project.
// SPDX-License-Identifier: Apache-2.0
package policymanager

import (
	"context"
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/veraison/services/config"
	"github.com/veraison/services/kvstore"
	"github.com/veraison/services/policy"
	"github.com/veraison/services/proto"
	mock_deps "github.com/veraison/services/vts/policymanager/mocks"
	"google.golang.org/protobuf/types/known/structpb"
)

func TestPolicyMgr_getPolicy_not_found(t *testing.T) {
	ctrl := gomock.NewController(t)

	store := mock_deps.NewMockIKVStore(ctrl)
	store.EXPECT().
		Get(gomock.Eq("opa://0/TPM_ENACTTRUST")).
		Return(nil, kvstore.ErrKeyNotFound)

	// Get the Mock Agent here
	agent := mock_deps.NewMockIAgent(ctrl)
	agent.EXPECT().GetBackendName().Return("opa")
	evStruct, err := structpb.NewStruct(nil)
	require.NoError(t, err)

	ec := &proto.EvidenceContext{
		Format:        proto.AttestationFormat_TPM_ENACTTRUST,
		TenantId:      "0",
		TrustAnchorId: "TPM_ENACTTRUST://0/7df7714e-aa04-4638-bcbf-434b1dd720f1",
		SoftwareId:    "TPM_ENACTTRUST://0/7df7714e-aa04-4638-bcbf-434b1dd720f1",
		Evidence:      evStruct,
	}

	pm := &PolicyManager{Store: store, Agent: agent}

	pol, err := pm.getPolicy(ec)
	assert.Nil(t, pol)
	assert.ErrorIs(t, err, ErrNoPolicy)
}

func TestPolicyMgr_getPolicy_mult_found(t *testing.T) {
	ctrl := gomock.NewController(t)

	store := mock_deps.NewMockIKVStore(ctrl)
	store.EXPECT().
		Get(gomock.Eq("opa://0/TPM_ENACTTRUST")).
		Return([]string{"Policy 1", "Policy 2"}, nil)

	// Get the Mock Agent here
	agent := mock_deps.NewMockIAgent(ctrl)
	agent.EXPECT().GetBackendName().Return("opa")
	evStruct, err := structpb.NewStruct(nil)
	require.NoError(t, err)

	ec := &proto.EvidenceContext{
		Format:        proto.AttestationFormat_TPM_ENACTTRUST,
		TenantId:      "0",
		TrustAnchorId: "TPM_ENACTTRUST://0/7df7714e-aa04-4638-bcbf-434b1dd720f1",
		SoftwareId:    "TPM_ENACTTRUST://0/7df7714e-aa04-4638-bcbf-434b1dd720f1",
		Evidence:      evStruct,
	}

	pm := &PolicyManager{Store: store, Agent: agent}

	pol, err := pm.getPolicy(ec)
	assert.Nil(t, pol)
	assert.EqualError(t, err, `found 2 policy entries for id "opa://0/TPM_ENACTTRUST"; must be at most 1`)
}
func TestPolicyMgr_getPolicy_OK(t *testing.T) {
	ctrl := gomock.NewController(t)

	store := mock_deps.NewMockIKVStore(ctrl)
	store.EXPECT().
		Get(gomock.Eq("opa://0/TPM_ENACTTRUST")).
		Return([]string{"Real Value"}, nil)

	agent := mock_deps.NewMockIAgent(ctrl)
	agent.EXPECT().GetBackendName().Return("opa")
	evStruct, err := structpb.NewStruct(nil)
	require.NoError(t, err)

	ec := &proto.EvidenceContext{
		Format:        proto.AttestationFormat_TPM_ENACTTRUST,
		TenantId:      "0",
		TrustAnchorId: "TPM_ENACTTRUST://0/7df7714e-aa04-4638-bcbf-434b1dd720f1",
		SoftwareId:    "TPM_ENACTTRUST://0/7df7714e-aa04-4638-bcbf-434b1dd720f1",
		Evidence:      evStruct,
	}

	pm := &PolicyManager{Store: store, Agent: agent}

	_, err = pm.getPolicy(ec)
	require.NoError(t, err)
}

func TestPolicyMgr_New_policyAgent_OK(t *testing.T) {
	ctrl := gomock.NewController(t)

	store := mock_deps.NewMockIKVStore(ctrl)
	cfg := config.Store{
		policy.DirectiveBackend: "opa",
	}
	_, err := New(cfg, store)
	require.NoError(t, err)
}

func TestPolicyMgr_New_policyAgent_NOK(t *testing.T) {
	ctrl := gomock.NewController(t)

	store := mock_deps.NewMockIKVStore(ctrl)
	cfg := config.Store{
		policy.DirectiveBackend: "nope",
	}
	_, err := New(cfg, store)
	assert.EqualError(t, err, `backend "nope" is not supported`)
}

func TestPolicyMgr_Evaluate_OK(t *testing.T) {
	ctrl := gomock.NewController(t)
	evStruct, _ := structpb.NewStruct(nil)

	store := mock_deps.NewMockIKVStore(ctrl)
	store.EXPECT().
		Get(gomock.Eq("opa://0/TPM_ENACTTRUST")).
		Return([]string{"Real Value"}, nil)

	ec := &proto.EvidenceContext{
		Format:        proto.AttestationFormat_TPM_ENACTTRUST,
		TenantId:      "0",
		TrustAnchorId: "TPM_ENACTTRUST://0/7df7714e-aa04-4638-bcbf-434b1dd720f1",
		SoftwareId:    "TPM_ENACTTRUST://0/7df7714e-aa04-4638-bcbf-434b1dd720f1",
		Evidence:      evStruct,
	}
	endorsements := []string{"h0KPxSKAPTEGXnvOPPA/5HUJZjHl4Hu9eg/eYMTPJcc="}
	ar := &proto.AttestationResult{}
	ac := &proto.AppraisalContext{Evidence: ec, Result: ar}

	agent := mock_deps.NewMockIAgent(ctrl)
	agent.EXPECT().GetBackendName().Return("opa")
	agent.EXPECT().Evaluate(context.TODO(), gomock.Any(), ac.Result, ec, endorsements)
	pm := &PolicyManager{Store: store, Agent: agent}
	err := pm.Evaluate(context.TODO(), ac, endorsements)
	require.NoError(t, err)
}

func TestPolicyMgr_Evaluate_NOK(t *testing.T) {
	ctrl := gomock.NewController(t)
	evStruct, _ := structpb.NewStruct(nil)

	store := mock_deps.NewMockIKVStore(ctrl)
	store.EXPECT().
		Get(gomock.Eq("opa://0/TPM_ENACTTRUST")).
		Return([]string{"Real Value"}, nil)

	ec := &proto.EvidenceContext{
		Format:        proto.AttestationFormat_TPM_ENACTTRUST,
		TenantId:      "0",
		TrustAnchorId: "TPM_ENACTTRUST://0/7df7714e-aa04-4638-bcbf-434b1dd720f1",
		SoftwareId:    "TPM_ENACTTRUST://0/7df7714e-aa04-4638-bcbf-434b1dd720f1",
		Evidence:      evStruct,
	}
	endorsements := []string{"h0KPxSKAPTEGXnvOPPA/5HUJZjHl4Hu9eg/eYMTPJcc="}
	ar := &proto.AttestationResult{}
	ac := &proto.AppraisalContext{Evidence: ec, Result: ar}
	expectedErr := errors.New("could not evaluate policy: policy returned bad update")
	agent := mock_deps.NewMockIAgent(ctrl)
	agent.EXPECT().GetBackendName().Return("opa")
	agent.EXPECT().Evaluate(context.TODO(), gomock.Any(), ac.Result, ec, endorsements).Return(nil, expectedErr)
	pm := &PolicyManager{Store: store, Agent: agent}
	err := pm.Evaluate(context.TODO(), ac, endorsements)
	assert.ErrorIs(t, err, expectedErr)

}