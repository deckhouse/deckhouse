/*
Copyright 2025 Flant JSC

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

package moduleconfig

import (
	"fmt"
	"regexp"
	"time"
)

var (
	ttlMin    = 5 * time.Minute
	ttlRegexp = regexp.MustCompile(`^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$`)
)

func validateTTL(ttl string) error {
	if len(ttl) > 0 {
		if !ttlRegexp.MatchString(ttl) {
			return fmt.Errorf("does not match required pattern %q", ttlRegexp.String())
		}

		duration, err := time.ParseDuration(ttl)
		if err != nil {
			return err
		}

		if duration < ttlMin {
			return fmt.Errorf("must be at least %q", ttlMin.String())
		}
	}
	return nil
}
