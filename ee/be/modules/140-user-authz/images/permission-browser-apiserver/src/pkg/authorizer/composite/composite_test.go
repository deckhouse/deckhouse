/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package composite

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apiserver/pkg/authorization/authorizer"
)

// mockAuthorizer implements authorizer.Authorizer for testing
type mockAuthorizer struct {
	decision authorizer.Decision
	reason   string
	err      error
}

func (m *mockAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	return m.decision, m.reason, m.err
}

func TestCompositeAuthorizer_MultitenancyDenies(t *testing.T) {
	mt := &mockAuthorizer{decision: authorizer.DecisionDeny, reason: "namespace restricted"}
	rbac := &mockAuthorizer{decision: authorizer.DecisionAllow, reason: "RBAC allowed"}

	c := NewCompositeAuthorizer(mt, rbac)
	decision, reason, err := c.Authorize(context.Background(), nil)

	assert.NoError(t, err)
	assert.Equal(t, authorizer.DecisionDeny, decision)
	assert.Equal(t, "namespace restricted", reason)
}

func TestCompositeAuthorizer_MultitenancyNoOpinion_RBACAllows(t *testing.T) {
	mt := &mockAuthorizer{decision: authorizer.DecisionNoOpinion}
	rbac := &mockAuthorizer{decision: authorizer.DecisionAllow, reason: "RBAC allowed"}

	c := NewCompositeAuthorizer(mt, rbac)
	decision, reason, err := c.Authorize(context.Background(), nil)

	assert.NoError(t, err)
	assert.Equal(t, authorizer.DecisionAllow, decision)
	assert.Equal(t, "RBAC allowed", reason)
}

func TestCompositeAuthorizer_MultitenancyNoOpinion_RBACDenies(t *testing.T) {
	mt := &mockAuthorizer{decision: authorizer.DecisionNoOpinion}
	rbac := &mockAuthorizer{decision: authorizer.DecisionDeny, reason: "RBAC denied"}

	c := NewCompositeAuthorizer(mt, rbac)
	decision, reason, err := c.Authorize(context.Background(), nil)

	assert.NoError(t, err)
	assert.Equal(t, authorizer.DecisionDeny, decision)
	assert.Equal(t, "RBAC denied", reason)
}

func TestCompositeAuthorizer_MultitenancyError(t *testing.T) {
	mt := &mockAuthorizer{err: errors.New("mt error")}
	rbac := &mockAuthorizer{decision: authorizer.DecisionAllow}

	c := NewCompositeAuthorizer(mt, rbac)
	decision, _, err := c.Authorize(context.Background(), nil)

	assert.Error(t, err)
	assert.Equal(t, authorizer.DecisionNoOpinion, decision)
}

func TestCompositeAuthorizer_RBACError(t *testing.T) {
	mt := &mockAuthorizer{decision: authorizer.DecisionNoOpinion}
	rbac := &mockAuthorizer{err: errors.New("rbac error")}

	c := NewCompositeAuthorizer(mt, rbac)
	decision, _, err := c.Authorize(context.Background(), nil)

	assert.Error(t, err)
	assert.Equal(t, authorizer.DecisionNoOpinion, decision)
}

func TestCompositeAuthorizer_NilMultitenancy(t *testing.T) {
	rbac := &mockAuthorizer{decision: authorizer.DecisionAllow, reason: "RBAC allowed"}

	c := NewCompositeAuthorizer(nil, rbac)
	decision, reason, err := c.Authorize(context.Background(), nil)

	assert.NoError(t, err)
	assert.Equal(t, authorizer.DecisionAllow, decision)
	assert.Equal(t, "RBAC allowed", reason)
}

func TestCompositeAuthorizer_BothNoOpinion(t *testing.T) {
	mt := &mockAuthorizer{decision: authorizer.DecisionNoOpinion}
	rbac := &mockAuthorizer{decision: authorizer.DecisionNoOpinion}

	c := NewCompositeAuthorizer(mt, rbac)
	decision, reason, err := c.Authorize(context.Background(), nil)

	assert.NoError(t, err)
	assert.Equal(t, authorizer.DecisionNoOpinion, decision)
	assert.Empty(t, reason)
}
