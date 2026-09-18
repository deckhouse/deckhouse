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

// Package addonutils is the single point where this repository depends on
// addon-operator's values machinery.
//
// Everything here is a re-export: the types are aliases, so a value of
// [Values] is the very same type addon-operator's own API takes and returns and
// crosses that boundary with no conversion. Nothing changes at runtime, and
// nothing is meant to: the package exists so the dependency has one seam instead
// of fifty, and replacing the implementation later is a change to this file
// rather than to every caller.
//
// Only the surface this repository actually uses is re-exported. Widen it when a
// caller needs more, rather than mirroring upstream wholesale — the narrower this
// file stays, the smaller the eventual replacement is.
//
// Note that this is not yet a decoupling: addon-operator stays in the build
// regardless, since the package runtime still drives its module manager, hooks
// and schema validation.
package addonutils

import (
	"os"

	"github.com/flant/addon-operator/pkg/utils"
)

// Values is a package's value tree: the user configuration merged with the
// defaults of its OpenAPI schema, as handed to hooks and Helm.
type Values = utils.Values

// ValuesPatch is a set of JSON Patch operations a hook returned against a
// value tree.
type ValuesPatch = utils.ValuesPatch

// ValuesPatchType names the value tree a patch applies to.
type ValuesPatchType = utils.ValuesPatchType

// ApplyPatchMode decides what happens when a patch addresses a path that the
// value tree does not have.
type ApplyPatchMode = utils.ApplyPatchMode

// Patch is a raw list of JSON Patch operations, before it is bound to a
// particular value tree.
type Patch = utils.Patch

// ModuleConfig is a module's configuration as addon-operator's kube config
// manager holds it.
type ModuleConfig = utils.ModuleConfig

// Maintenance is a module's maintenance mode.
type Maintenance = utils.Maintenance

const (
	// GlobalValuesKey is the key the global values live under.
	GlobalValuesKey = utils.GlobalValuesKey

	// ValuesFileName is the name of a package's static values file.
	ValuesFileName = utils.ValuesFileName

	// MemoryValuesPatch is the patch a hook applies to the in-memory values,
	// as opposed to the configuration the user owns.
	MemoryValuesPatch = utils.MemoryValuesPatch

	// Strict fails a patch that addresses a path the value tree does not have.
	Strict = utils.Strict

	// IgnoreNonExistentPaths skips such an operation instead of failing.
	IgnoreNonExistentPaths = utils.IgnoreNonExistentPaths
)

// NewModuleConfig returns a module configuration holding the given values.
func NewModuleConfig(moduleName string, values Values) *ModuleConfig {
	return utils.NewModuleConfig(moduleName, values)
}

// ModuleNameToValuesKey converts a module name to the camelCase key its values
// live under.
func ModuleNameToValuesKey(moduleName string) string {
	return utils.ModuleNameToValuesKey(moduleName)
}

// ModuleNameFromValuesKey converts a camelCase values key back to the module name.
func ModuleNameFromValuesKey(moduleValuesKey string) string {
	return utils.ModuleNameFromValuesKey(moduleValuesKey)
}

// MergeValues deep-merges value trees, with later arguments winning.
func MergeValues(values ...Values) Values {
	return utils.MergeValues(values...)
}

// NewValuesFromBytes parses a value tree from YAML.
func NewValuesFromBytes(data []byte) (Values, error) {
	return utils.NewValuesFromBytes(data)
}

// NewValuesPatch returns an empty patch to append operations to.
func NewValuesPatch() *ValuesPatch {
	return utils.NewValuesPatch()
}

// ApplyValuesPatch applies a patch to a value tree, reporting whether it changed
// anything.
func ApplyValuesPatch(values Values, valuesPatch ValuesPatch, mode ApplyPatchMode) (Values, bool, error) {
	return utils.ApplyValuesPatch(values, valuesPatch, mode)
}

// AppendValuesPatch adds a patch to a list, collapsing it into the previous one
// where it can.
func AppendValuesPatch(valuesPatches []ValuesPatch, newValuesPatch ValuesPatch) []ValuesPatch {
	return utils.AppendValuesPatch(valuesPatches, newValuesPatch)
}

// JsonPatchFromBytes parses a raw JSON Patch document.
//
//nolint:revive // named for the upstream function it re-exports.
func JsonPatchFromBytes(data []byte) (Patch, error) {
	return utils.JsonPatchFromBytes(data)
}

// ReadOpenAPIFiles reads a package's config-values and values schemas from its
// openapi directory.
func ReadOpenAPIFiles(openAPIDir string) ([]byte, []byte, error) {
	return utils.ReadOpenAPIFiles(openAPIDir)
}

// LoadValuesFileFromDir reads a package's static values file.
func LoadValuesFileFromDir(dir string, strictModeEnabled bool) (Values, error) {
	return utils.LoadValuesFileFromDir(dir, strictModeEnabled)
}

// DumpData writes data to a file, creating the parent directories it needs.
func DumpData(filePath string, data []byte) error {
	return utils.DumpData(filePath, data)
}

// CalculateStringsChecksum returns a checksum over the given strings.
func CalculateStringsChecksum(stringArr ...string) string {
	return utils.CalculateStringsChecksum(stringArr...)
}

// SplitToPaths splits a colon-separated list of directories.
func SplitToPaths(dir string) []string {
	return utils.SplitToPaths(dir)
}

// SymlinkInfo resolves a directory entry that may be a symlink, reporting the
// target and whether it is a directory.
func SymlinkInfo(path string, info os.FileInfo) (string, bool, error) {
	return utils.SymlinkInfo(path, info)
}
