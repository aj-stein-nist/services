// Copyright 2022-2023 Contributors to the Veraison project.
// SPDX-License-Identifier: Apache-2.0
package policy

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/veraison/services/log"
)

func Test_Store_CRUD(t *testing.T) {
	v := viper.New()
	v.Set("backend", "memory")

	store, err := NewStore(v, log.Named("test"))
	require.NoError(t, err)
	defer store.Close()

	pid := PolicyID{"1", "scheme", "policy"}

	err = store.Add(pid, "1. the chief's always right; 2. if the chief's wrong, see 1.")
	require.NoError(t, err)

	policy, err := store.GetLatest(pid)
	require.NoError(t, err)

	assert.Equal(t, pid, policy.ID)
	assert.Equal(t, int32(1), policy.Version)

	err = store.Add(pid, "On second thought, chief's not always right.")
	assert.ErrorContains(t, err, "already exists")

	err = store.Update(pid, "On second thought, chief's not always right.")
	require.NoError(t, err)

	policy, err = store.GetLatest(pid)
	require.NoError(t, err)
	assert.Equal(t, int32(2), policy.Version)
	assert.Equal(t, "On second thought, chief's not always right.", policy.Rules)

	versions, err := store.Get(pid)
	require.NoError(t, err)
	assert.Len(t, versions, 2)
	assert.Equal(t, int32(2), versions[1].Version)

	policies, err := store.List()
	require.NoError(t, err)
	assert.Equal(t, 1, len(policies))
	assert.Equal(t, pid, policies[0].ID)

	policies, err = store.ListAllVersions()
	require.NoError(t, err)
	assert.Equal(t, 2, len(policies))
	assert.Equal(t, pid, policies[0].ID)
	assert.Equal(t, int32(1), policies[0].Version)
	assert.Equal(t, int32(2), policies[1].Version)

	err = store.Del(pid)
	require.NoError(t, err)

	_, err = store.GetLatest(pid)
	assert.ErrorIs(t, err, ErrNoPolicy)
}
