// Copyright 2026 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// ApplyEnvironment sets every flag of cmd that argv left unset from its DHCTL_CLI_* variable; an
// empty variable counts as unset. The flag is marked changed, as argv marks it. Then a required
// flag left unset fails the run here, ahead of the PreRunE hooks: cobra checks it only after them.
func ApplyEnvironment(cmd *cobra.Command) error {
	for _, f := range flagList(cmd.Flags()) {
		value := os.Getenv(envOf(f))
		if f.Changed || value == "" {
			continue
		}

		if err := setFromEnv(f, value); err != nil {
			return fmt.Errorf("%s for --%s: %w", envOf(f), f.Name, err)
		}

		f.Changed = true
	}

	return cmd.ValidateRequiredFlags()
}

func flagList(flags *pflag.FlagSet) []*pflag.Flag {
	var out []*pflag.Flag

	flags.VisitAll(func(f *pflag.Flag) { out = append(out, f) })

	return out
}

// setFromEnv feeds the variable to the flag; a list or a map takes one item per line.
func setFromEnv(f *pflag.Flag, value string) error {
	switch f.Value.Type() {
	case flagTypeList, flagTypeSlice, flagTypePair:
		for _, item := range splitEnvList(value) {
			if err := f.Value.Set(item); err != nil {
				return err
			}
		}

		return nil
	}

	return f.Value.Set(value)
}
