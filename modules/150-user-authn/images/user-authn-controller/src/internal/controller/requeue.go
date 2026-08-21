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

package controller

import "time"

// RequeueAfterTime is the delay until until. Zero if until is nil or not after now.
// Password and OfflineSessions do not change when a lock expires, so watchers
// never fire; callers must requeue to strip status and the admin-lock annotation.
func RequeueAfterTime(until *time.Time, now time.Time) time.Duration {
	if until == nil || !until.After(now) {
		return 0
	}
	return until.Sub(now)
}
