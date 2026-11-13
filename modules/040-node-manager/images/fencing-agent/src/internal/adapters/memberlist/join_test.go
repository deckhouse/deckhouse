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

package memberlist

import (
	"errors"
	"fmt"
	"testing"
	"testing/synctest"
	"time"
)

func TestJoinEachJoinsTheSeedsConcurrently(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		join := func(string) (int, error) {
			time.Sleep(5 * time.Second)
			return 1, nil
		}

		start := time.Now()
		joined, err := joinEach([]string{"a", "b", "c"}, join)

		if joined != 3 || err != nil {
			t.Errorf("joinEach() = (%d, %v), want (3, nil)", joined, err)
		}
		if elapsed := time.Since(start); elapsed != 5*time.Second {
			t.Errorf("joinEach() took %s, want 5s: the seeds are joined at once", elapsed)
		}
	})
}

func TestJoinEachSucceedsWhenOneSeedSucceeds(t *testing.T) {
	join := func(seed string) (int, error) {
		if seed == "b" {
			return 1, nil
		}
		return 0, errors.New("connection refused")
	}

	if joined, err := joinEach([]string{"a", "b", "c"}, join); joined != 1 || err != nil {
		t.Errorf("joinEach() = (%d, %v), want (1, nil)", joined, err)
	}
}

func TestJoinEachKeepsTheErrorsInSeedOrder(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		delays := map[string]time.Duration{"a": 3 * time.Second, "b": 2 * time.Second, "c": time.Second}
		join := func(seed string) (int, error) {
			time.Sleep(delays[seed])
			return 0, fmt.Errorf("failed to join %s", seed)
		}

		_, err := joinEach([]string{"a", "b", "c"}, join)

		want := "failed to join a\nfailed to join b\nfailed to join c"
		if err == nil || err.Error() != want {
			t.Errorf("joinEach() error = %v, want %q", err, want)
		}
	})
}

func TestJoinEachWithoutSeeds(t *testing.T) {
	if joined, err := joinEach(nil, nil); joined != 0 || err != nil {
		t.Errorf("joinEach(nil) = (%d, %v), want (0, nil)", joined, err)
	}
}
