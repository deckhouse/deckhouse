package operation

// MetricAction defines the supported metric operation types
type MetricAction string

// Enum values for MetricAction
const (
	ActionSet     MetricAction = "set"
	ActionAdd     MetricAction = "add"
	ActionObserve MetricAction = "observe"
	ActionExpire  MetricAction = "expire"
)

// IsValid checks if the action is one of the valid actions
func (a MetricAction) IsValid() bool {
	switch a {
	case ActionSet, ActionAdd, ActionObserve, ActionExpire:
		return true
	default:
		return false
	}
}

// String returns the string representation of the MetricAction
func (a MetricAction) String() string {
	return string(a)
}
