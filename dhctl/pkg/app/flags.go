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
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Flag declares one dhctl flag. Env names the variable the flag reads, kept in a pflag annotation;
// Required means argv or the environment must give it.
// The first back-quoted word of Help is the value placeholder in --help (pflag.UnquoteUsage).
type Flag struct {
	Name     string
	Short    string
	Env      string
	Help     string
	Required bool
	Hidden   bool
}

const envAnnotation = "dhctl.env"

// Value type names as pflag reports them; the environment apply switches on them.
const (
	FlagTypeBool  = "bool"
	flagTypeList  = "stringArray"
	flagTypeSlice = "stringSlice"
	flagTypePair  = "pair"
	flagTypeEnum  = "enum"
)

// The default of every flag is the value its target holds at registration. A flag of one value
// refuses a second one on argv, as kingpin did; a list or a map takes every occurrence.

func StringFlag(flags *pflag.FlagSet, f Flag, target *string) {
	flags.StringVarP(target, f.Name, f.Short, *target, f.Help)
	annotate(flags, f)
	refuseRepeat(flags, f.Name)
}

func StringsFlag(flags *pflag.FlagSet, f Flag, target *[]string) {
	flags.StringArrayVarP(target, f.Name, f.Short, *target, f.Help)
	annotate(flags, f)
}

// StringSliceFlag splits every value on commas too, as lib-connection's list flags did.
func StringSliceFlag(flags *pflag.FlagSet, f Flag, target *[]string) {
	flags.StringSliceVarP(target, f.Name, f.Short, *target, f.Help)
	annotate(flags, f)
}

// BoolFlag also takes a hidden --no-<name>, as kingpin did.
func BoolFlag(flags *pflag.FlagSet, f Flag, target *bool) {
	flags.BoolVarP(target, f.Name, f.Short, *target, f.Help)
	annotate(flags, f)
	refuseRepeat(flags, f.Name)
	negation := flags.VarPF(negationValue{flag: flags.Lookup(f.Name)}, "no-"+f.Name, "", "")
	negation.NoOptDefVal = "true"
	negation.Hidden = true
}

func IntFlag(flags *pflag.FlagSet, f Flag, target *int) {
	flags.IntVarP(target, f.Name, f.Short, *target, f.Help)
	annotate(flags, f)
	refuseRepeat(flags, f.Name)
}

func DurationFlag(flags *pflag.FlagSet, f Flag, target *time.Duration) {
	flags.DurationVarP(target, f.Name, f.Short, *target, f.Help)
	annotate(flags, f)
	refuseRepeat(flags, f.Name)
}

func EnumFlag(flags *pflag.FlagSet, f Flag, target *string, options ...string) {
	flags.VarP(&enumValue{target: target, options: options}, f.Name, f.Short, f.Help)
	annotate(flags, f)
	refuseRepeat(flags, f.Name)
}

// PairsFlag takes one KEY=VALUE per occurrence and keeps commas in the value.
func PairsFlag(flags *pflag.FlagSet, f Flag, target *map[string]string) {
	flags.VarP(&pairValue{target: target}, f.Name, f.Short, f.Help)
	annotate(flags, f)
}

func annotate(flags *pflag.FlagSet, f Flag) {
	if f.Env != "" {
		mustFlag(f.Name, flags.SetAnnotation(f.Name, envAnnotation, []string{f.Env}))
	}

	if f.Required {
		mustFlag(f.Name, cobra.MarkFlagRequired(flags, f.Name))
	}

	if f.Hidden {
		mustFlag(f.Name, flags.MarkHidden(f.Name))
	}
}

func mustFlag(name string, err error) {
	if err != nil {
		panic(fmt.Sprintf("flag %q: %v", name, err))
	}
}

// refuseRepeat wraps the value of a registered flag. The environment sets only a flag argv left
// unchanged, so it sets it once too.
func refuseRepeat(flags *pflag.FlagSet, name string) {
	f := flags.Lookup(name)
	f.Value = onceValue{Value: f.Value, flag: f}
}

type onceValue struct {
	pflag.Value
	flag *pflag.Flag
}

var _ pflag.Value = onceValue{}

func (v onceValue) Set(s string) error {
	if v.flag.Changed {
		return repeatedFlagError(v.flag.Name)
	}

	return v.Value.Set(s)
}

type repeatedFlagError string

var _ error = repeatedFlagError("")

func (e repeatedFlagError) Error() string {
	return fmt.Sprintf("flag '%s' cannot be repeated", string(e))
}

// KingpinFlagNames keeps the names kingpin took for help and version: --help-long, and --no-help
// and --no-version, which kingpin acted on as on the flag itself. A cobra normalization function.
func KingpinFlagNames(_ *pflag.FlagSet, name string) pflag.NormalizedName {
	switch name {
	case "help-long", "no-help", "no-help-long":
		return "help"
	case "no-version":
		return "version"
	}

	return pflag.NormalizedName(name)
}

// FlagError is a flag parse error as kingpin worded it: a repeated flag without the
// `invalid argument "b" for "--ssh-user" flag:` that pflag puts before an error of a value.
// A cobra flag error function.
func FlagError(_ *cobra.Command, err error) error {
	if repeated, ok := errors.AsType[repeatedFlagError](err); ok {
		return repeated
	}

	return err
}

// negationValue is --no-<name> of a bool flag: it sets the flag to false and counts as giving it.
type negationValue struct {
	flag *pflag.Flag
}

var _ pflag.Value = negationValue{}

func (v negationValue) String() string { return "false" }
func (v negationValue) Type() string   { return FlagTypeBool }
func (v negationValue) Set(s string) error {
	negated, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}

	if err := v.flag.Value.Set(strconv.FormatBool(!negated)); err != nil {
		return err
	}

	v.flag.Changed = true

	return nil
}

func envOf(f *pflag.Flag) string {
	if v := f.Annotations[envAnnotation]; len(v) > 0 {
		return v[0]
	}

	return ""
}

// ShowEnvInHelp makes the help of cmd and of its subcommands name the variable of a flag after its
// help text, as kingpin FlagModel.HelpWithEnvar did. The help is rendered from copies of the flags,
// so Usage stays as declared.
func ShowEnvInHelp(cmd *cobra.Command) {
	cobra.AddTemplateFunc("flagUsagesWithEnv", flagUsagesWithEnv)
	cmd.SetUsageTemplate(strings.NewReplacer(
		".LocalFlags.FlagUsages", "flagUsagesWithEnv .LocalFlags",
		".InheritedFlags.FlagUsages", "flagUsagesWithEnv .InheritedFlags",
	).Replace(cmd.UsageTemplate()))
}

func flagUsagesWithEnv(flags *pflag.FlagSet) string {
	shown := pflag.NewFlagSet("", pflag.ContinueOnError)
	shown.SortFlags = flags.SortFlags
	flags.VisitAll(func(f *pflag.Flag) {
		withEnv := *f
		if env := envOf(f); env != "" {
			withEnv.Usage = fmt.Sprintf("%s ($%s)", f.Usage, env)
		}

		shown.AddFlag(&withEnv)
	})

	return shown.FlagUsages()
}

type enumValue struct {
	target  *string
	options []string
}

var _ pflag.Value = (*enumValue)(nil)

func (v *enumValue) String() string { return *v.target }
func (v *enumValue) Type() string   { return flagTypeEnum }
func (v *enumValue) Set(s string) error {
	if !slices.Contains(v.options, s) {
		return fmt.Errorf("enum value must be one of %s, got '%s'", strings.Join(v.options, ","), s)
	}

	*v.target = s

	return nil
}

type pairValue struct {
	target *map[string]string
}

var _ pflag.Value = (*pairValue)(nil)

func (p *pairValue) String() string {
	if len(*p.target) == 0 {
		return ""
	}

	return fmt.Sprint(*p.target)
}

func (p *pairValue) Type() string { return flagTypePair }

func (p *pairValue) Set(s string) error {
	k, v, ok := strings.Cut(s, "=")
	if !ok {
		return fmt.Errorf("expected KEY=VALUE got '%s'", s)
	}

	if *p.target == nil {
		*p.target = map[string]string{}
	}

	(*p.target)[k] = v

	return nil
}

// splitEnvList splits a list given in one variable: one item per line, a trailing newline
// ignored. Like alecthomas/kingpin envar.go GetSplitEnvarValue.
func splitEnvList(s string) []string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}
