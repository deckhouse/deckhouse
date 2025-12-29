package condmapper

// Rule defines how to compute an external condition from internal state.
type Rule struct {
	Type    string // external condition type name
	TrueIf  Pred   // set True when matched; source used for Reason/Message
	FalseIf Pred   // set False when matched; source used for Reason/Message
	OnlyIf  Pred   // precondition; skip rule if not matched
	Sticky  bool   // once True, stays True forever
}
