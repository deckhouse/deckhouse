package check

// Checker defines the common interface for check operations
//
// Do not re-use checkers, always create new ones. Usually checkers are a stateful composition of other checkers.
// They are stateful due to BusyWith method. See SequentialChecker as an example.
type Checker interface {
	// Check does the actual job to determine the result. Returns nil if everything is ok.
	Check() Error

	// BusyWith describes what the check is doing. Used in logging and possibly other details of the
	// probe flow.
	BusyWith() string
}
