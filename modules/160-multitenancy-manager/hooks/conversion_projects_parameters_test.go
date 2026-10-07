/*
Copyright 2026 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import "testing"

// v1alpha2ParametersFixture is a golden fixture of the v1alpha3 -> v1alpha2 conversion for projects as
// the controller writes them back. The controller renders a Helm resourcesTemplate from the parameters
// a v1alpha2 reader gets, which it computes in Go, and its validate tests check that copy against this
// file. The file has a directory of its own, since TestConversionBindings takes every file of the
// projects directory for a conversion path of the hook.
const v1alpha2ParametersFixture = conversionFixtures + "/projects/parameters/v1alpha3_to_v1alpha2.yaml"

// The down-conversion gives a v1alpha2 reader the parameters of the fixture, with every jq engine, so
// the Go copy of it that the controller renders from is held to the conversion the cluster runs. This
// test is kept out of conversion_projects_test.go, which the previous release branch carries byte for
// byte without the controller code the fixture is for.
func TestProjectDownConversionParameters(t *testing.T) {
	t.Parallel()

	testConversionGolden(t, conversionHook, v1alpha2ParametersFixture)
}
