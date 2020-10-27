package api

import (
	"testing"

	. "github.com/onsi/gomega"
)

func Test_DecodeFromToStep(t *testing.T) {
	g := NewWithT(t)

	from, to, step, err := DecodeFromToStep([]string{"300"}, []string{"600"}, []string{"1m"})
	g.Expect(err).ShouldNot(HaveOccurred())
	g.Expect(from).Should(BeEquivalentTo(300))
	g.Expect(to).Should(BeEquivalentTo(600))
	g.Expect(step).Should(BeEquivalentTo(60))
}
