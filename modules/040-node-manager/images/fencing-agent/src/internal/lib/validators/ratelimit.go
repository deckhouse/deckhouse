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

package validators

import "fmt"

func ValidateRateLimit(rps, burst int, name string) error {
	if rps > 0 && burst == 0 {
		return fmt.Errorf("%s: burst must be > 0 when RPS > 0", name)
	}
	if rps > 0 && burst < rps {
		return fmt.Errorf("%s: burst should be >= RPS", name)
	}
	if burst > 0 && rps == 0 {
		return fmt.Errorf("%s: RPS must be > 0 when burst > 0", name)
	}
	return nil
}
