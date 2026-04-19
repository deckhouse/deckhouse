/*
Copyright 2025 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package composite

import (
	"context"

	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/klog/v2"
)

// CompositeAuthorizer combines multi-tenancy and RBAC authorization
// Order of operations:
// 1. Multi-tenancy layer (only denies, never allows)
// 2. RBAC layer (allows or denies)
type CompositeAuthorizer struct {
	multitenancy authorizer.Authorizer
	rbac         authorizer.Authorizer
}

// NewCompositeAuthorizer creates a new composite authorizer
func NewCompositeAuthorizer(mt, rbac authorizer.Authorizer) *CompositeAuthorizer {
	return &CompositeAuthorizer{
		multitenancy: mt,
		rbac:         rbac,
	}
}

// Authorize implements authorizer.Authorizer
func (c *CompositeAuthorizer) Authorize(ctx context.Context, attrs authorizer.Attributes) (authorizer.Decision, string, error) {
	// 1) Multi-tenancy layer (only denies; Allow is never returned)
	if c.multitenancy != nil {
		decision, reason, err := c.multitenancy.Authorize(ctx, attrs)
		if err != nil {
			klog.V(4).Infof("Multi-tenancy authorizer error: %v", err)
			return authorizer.DecisionNoOpinion, "", err
		}
		if decision == authorizer.DecisionDeny {
			klog.V(4).Infof("Multi-tenancy denied: %s", reason)
			return authorizer.DecisionDeny, reason, nil
		}
	}

	// 2) RBAC layer
	decision, reason, err := c.rbac.Authorize(ctx, attrs)
	if err != nil {
		klog.V(4).Infof("RBAC authorizer error: %v", err)
		return authorizer.DecisionNoOpinion, "", err
	}

	if decision != authorizer.DecisionNoOpinion {
		klog.V(4).Infof("RBAC decision: %v, reason: %s", decision, reason)
		return decision, reason, nil
	}

	// 3) If RBAC says NoOpinion and multi-tenancy didn't deny, return NoOpinion
	return authorizer.DecisionNoOpinion, "", nil
}
