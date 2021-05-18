package api

import (
	"testing"

	. "github.com/onsi/gomega"

	"d8.io/upmeter/pkg/server/ranges"
)

func Test_parseStepRange(t *testing.T) {
	g := NewWithT(t)

	cases := []struct {
		args [3]string
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
	}

	for _, tt := range cases {
		r, err := parseStepRange(tt.args[0], tt.args[1], tt.args[2])

		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(r.From).Should(BeEquivalentTo(tt.want.From))
		g.Expect(r.To).Should(BeEquivalentTo(tt.want.To))
		g.Expect(r.Step).Should(BeEquivalentTo(tt.want.Step))
	}
}
