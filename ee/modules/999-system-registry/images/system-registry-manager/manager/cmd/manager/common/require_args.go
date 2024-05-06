package common

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

// NoArgs returns an error if any args are included.
func NoArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return errors.Errorf(
			"%q accepts no arguments\n\nUsage:  %s",
			cmd.CommandPath(),
			cmd.UseLine(),
		)
	}
	return nil
}

// ExactArgs returns an error if there are not exactly n args.
func ExactArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) != n {
			return errors.Errorf(
				"%q requires %d argument\n\nUsage:  %s",
				cmd.CommandPath(),
				n,
				cmd.UseLine(),
			)
		}
		return nil
	}
}

// MaximumNArgs returns an error if there are more than N args.
func MaximumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) > n {
			return errors.Errorf(
				"%q accepts at most %d argument\n\nUsage:  %s",
				cmd.CommandPath(),
				n,
				cmd.UseLine(),
			)
		}
		return nil
	}
}

// MinimumNArgs returns an error if there is not at least N args.
func MinimumNArgs(n int) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) < n {
			return errors.Errorf(
				"%q requires at least %d argument\n\nUsage:  %s",
				cmd.CommandPath(),
				n,
				cmd.UseLine(),
			)
		}
		return nil
	}
}
