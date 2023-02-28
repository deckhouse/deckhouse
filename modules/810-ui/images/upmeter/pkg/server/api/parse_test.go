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

package api

import (
	"testing"

	. "github.com/onsi/gomega"

	"d8.io/upmeter/pkg/server/ranges"
)

func Test_parseStepRange(t *testing.T) {
	g := NewWithT(t)

	cases := []struct {
		args [3]string // from, to, step
		want ranges.StepRange
	}{
		{
			// seconds notation in step
			args: [...]string{"1617061615", "1617083215", "1800"},
			want: ranges.StepRange{From: 1617061615, To: 1617083215, Step: 1800},
		},
		{
			// duration notation in step
			args: [...]string{"1617061615", "1617083215", "30m"},
			want: ranges.StepRange{From: 1617061615, To: 1617083215, Step: 1800},
		},
		{
			// seconds notation for small step
			args: [...]string{"1617061600", "1617083230", "30"},
			want: ranges.StepRange{From: 1617061600, To: 1617083230, Step: 30},
		},
	}

	for _, tt := range cases {
		r, err := parseStepRange(tt.args[0], tt.args[1], tt.args[2])

		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(r.From).Should(BeEquivalentTo(tt.want.From))
		g.Expect(r.To).Should(BeEquivalentTo(tt.want.To))
		g.Expect(r.Step).Should(BeEquivalentTo(tt.want.Step))
	}
}
