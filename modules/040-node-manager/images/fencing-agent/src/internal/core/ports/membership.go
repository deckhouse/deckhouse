package ports

import "fencing-controller/internal/core/domain"

type MembershipProvider interface {
	GetMembers() []domain.Node
	NumOtherMembers() int
	IsAlone() bool
}
