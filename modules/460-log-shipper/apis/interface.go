/*
Copyright 2021 Flant JSC

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

package apis

type LogSource interface {
	GetName() string
	// BuildSources in some cases you need to split source, for example: to match few namespaces
	// For the single log source - just return the input
	BuildSources() []LogSource
}

type LogTransform interface {
	GetName() string
	SetName(string)
	SetInputs([]string)
	GetInputs() []string
}

type LogDestination interface {
	GetName() string
	SetInputs([]string)
}
