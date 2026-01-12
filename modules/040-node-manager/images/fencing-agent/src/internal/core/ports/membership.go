package ports

import "fencing-agent/internal/core/domain"

type MembershipProvider interface {
	GetMembers() []domain.Node
	NumOtherMembers() int
	IsAlone() bool
}
