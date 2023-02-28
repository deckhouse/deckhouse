/*
Copyright 2023 Flant JSC

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

package check

// Checker defines the common interface for check operations
//
// Do not re-use checkers, always create new ones. Usually checkers are a stateful composition of other checkers.
// They are stateful due to BusyWith method. See SequentialChecker as an example.
type Checker interface {
	// Check does the actual job to determine the result. Returns nil if everything is ok.
	Check() Error
}
