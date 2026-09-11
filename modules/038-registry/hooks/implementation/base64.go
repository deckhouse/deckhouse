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

package implementation

import "encoding/base64"

// decodeBase64 keeps the state filter about the decision rather than about encodings: a Secret read
// through an unstructured object hands over its data still encoded.
func decodeBase64(raw string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(raw)
}
