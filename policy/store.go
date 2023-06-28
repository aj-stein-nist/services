// Copyright 2022-2023 Contributors to the Veraison project.
// SPDX-License-Identifier: Apache-2.0
package policy

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/spf13/viper"
	"github.com/veraison/services/kvstore"
	"go.uber.org/zap"
)

var ErrNoPolicy = errors.New("no policy found")

// NewStore returns a new policy store. Config options are the same as those
// used for kvstore.New().
func NewStore(v *viper.Viper, logger *zap.SugaredLogger) (*Store, error) {
	kvStore, err := kvstore.New(v, logger)
	if err != nil {
		return nil, err
	}

	return &Store{KVStore: kvStore, Logger: logger}, nil
}

type Store struct {
	KVStore kvstore.IKVStore
	Logger  *zap.SugaredLogger
}

// Setup the underyling kvstore. This is a one-time setup that only needs to be
// performed once for a deployment.
func (o *Store) Setup() error {
	return o.KVStore.Setup()
}

// Add a policy with the specified ID and rules. If a policy with that ID
// already exists, an error is returned.
func (o *Store) Add(id PolicyID, rules string) error {
	if _, err := o.Get(id); err == nil {
		return fmt.Errorf("policy with id %q already exists", id)
	}

	return o.Update(id, rules)
}

// Update sets the provided rules as the latest version of the policy with the
// specified ID. If a policy with that ID does not exist, it is created.
func (o *Store) Update(id PolicyID, rules string) error {
	var oldVersion int32

	oldPolicy, err := o.GetLatest(id)

	if err == nil { // nolint:gocritic
		oldVersion = oldPolicy.Version
	} else if errors.Is(err, ErrNoPolicy) {
		oldVersion = 0
	} else {
		return err
	}

	newPolicy := Policy{ID: id, Rules: rules, Version: oldVersion + 1}

	newPolicyBytes, err := json.Marshal(newPolicy)
	if err != nil {
		return err
	}

	return o.KVStore.Add(id.StoreKey(), string(newPolicyBytes))
}

// Get returns the slice of all Policies associated with the specified ID. Each
// Policy represents a different version of the same logical policy.
func (o *Store) Get(id PolicyID) ([]Policy, error) {
	vals, err := o.KVStore.Get(id.StoreKey())
	if err != nil {
		if errors.Is(err, kvstore.ErrKeyNotFound) {
			return nil, fmt.Errorf("%w: %q", ErrNoPolicy, id)
		}
		return nil, err
	}

	var policies []Policy // nolint:prealloc

	for _, v := range vals {
		var p Policy
		if err = json.Unmarshal([]byte(v), &p); err != nil {
			return nil, err
		}

		p.ID = id
		policies = append(policies, p)
	}

	return policies, nil
}

// List returns []Policy containing latest versions of all policies. All
// policies returned will have distinct IDs. In cases where multiple policies
// exist for one ID in the store, the latest version will be returned.
func (o *Store) List() ([]Policy, error) {
	keys, err := o.GetPolicyIDs()
	if err != nil {
		return nil, err
	}

	var policies []Policy // nolint:prealloc
	for _, key := range keys {
		policy, err := o.GetLatest(key)
		if err != nil {
			return nil, err
		}

		policies = append(policies, policy)
	}

	return policies, nil
}

// ListAllVersions returns a []Policy containing every policy entry in the
// underlying store, including multiple versions associated with a single
// policy ID.
func (o *Store) ListAllVersions() ([]Policy, error) {
	keys, err := o.GetPolicyIDs()
	if err != nil {
		return nil, err
	}

	var policies []Policy
	for _, key := range keys {
		versions, err := o.Get(key)
		if err != nil {
			return nil, err
		}

		policies = append(policies, versions...)
	}

	return policies, nil
}

// GetPolicyIDs returns a []PolicyID of the policies currently in the store.
func (o *Store) GetPolicyIDs() ([]PolicyID, error) {
	keys, err := o.KVStore.GetKeys()
	if err != nil {
		return nil, err
	}

	ids := make([]PolicyID, len(keys))
	for i, k := range keys {
		pid, err := PolicyIDFromStoreKey(k)
		if err != nil {
			return nil, fmt.Errorf("bad key in store: %w", err)
		}

		ids[i] = pid
	}

	return ids, nil
}

// GetLatest returns the latest version of the policy with the specified ID. If
// no such policy exists, a wrapped ErrNoPolicy is returned.
func (o *Store) GetLatest(id PolicyID) (Policy, error) {
	policies, err := o.Get(id)
	if err != nil {
		return Policy{}, err
	}

	return policies[len(policies)-1], nil
}

// Del removes all policy versions associated with the specified id.
func (o *Store) Del(id PolicyID) error {
	return o.KVStore.Del(id.StoreKey())
}

// Close the connection to the underlying kvstore.
func (o *Store) Close() error {
	return o.KVStore.Close()
}
