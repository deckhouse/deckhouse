package kinds

import (
	"testing"

	"github.com/flant/constraint_exporter/pkg/gatekeeper"
	"github.com/stretchr/testify/assert"
)

func TestDeduplicate(t *testing.T) {
	con := gatekeeper.Constraint{Spec: gatekeeper.ConstraintSpec{Match: gatekeeper.ConstraintMatch{
		Kinds: []gatekeeper.MatchKind{
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{"networking.k8s.io", "extensions"},
				Kinds:     []string{"Ingress"},
			},
		},
	}}}

	con2 := gatekeeper.Constraint{Spec: gatekeeper.ConstraintSpec{Match: gatekeeper.ConstraintMatch{
		Kinds: []gatekeeper.MatchKind{
			{
				APIGroups: []string{""},
				Kinds:     []string{"Pod"},
			},
			{
				APIGroups: []string{"extensions", "networking.k8s.io"},
				Kinds:     []string{"Ingress"},
			},
		},
	}}}

	res := deduplicateKinds([]gatekeeper.Constraint{con, con2})

	assert.Len(t, res, 2)
	assert.Equal(t, res[":Pod"], gatekeeper.MatchKind{APIGroups: []string{""}, Kinds: []string{"Pod"}})
	assert.Equal(t, res["extensions,networking.k8s.io:Ingress"], gatekeeper.MatchKind{APIGroups: []string{"extensions", "networking.k8s.io"}, Kinds: []string{"Ingress"}})
}
