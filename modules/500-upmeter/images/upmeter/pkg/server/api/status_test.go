package api

import (
	"testing"

	. "github.com/onsi/gomega"
)

func Test_DecodeFromToStep(t *testing.T) {
	g := NewWithT(t)

	cases := []struct {
		args [3]string
		want timerange
	}{
		{
			// seconds notation in step
			args: [...]string{"1617061615", "1617083215", "1800"},
			want: timerange{from: 1617061615, to: 1617083215, step: 1800},
		},
		{
			// duration notation in step
			args: [...]string{"1617061615", "1617083215", "30m"},
			want: timerange{from: 1617061615, to: 1617083215, step: 1800},
		},
	}

	for _, tt := range cases {
		r, err := DecodeFromToStep(tt.args[0], tt.args[1], tt.args[2])

		g.Expect(err).ShouldNot(HaveOccurred())
		g.Expect(r.from).Should(BeEquivalentTo(tt.want.from))
		g.Expect(r.to).Should(BeEquivalentTo(tt.want.to))
		g.Expect(r.step).Should(BeEquivalentTo(tt.want.step))
	}
}
