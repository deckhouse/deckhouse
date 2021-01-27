package entity

import (
	"testing"

	. "github.com/onsi/gomega"
)

func Test_Get5MinSlot(t *testing.T) {
	g := NewWithT(t)

	g.Expect(Calculate5MinSlot(298)).Should(BeEquivalentTo(0))
	g.Expect(Calculate5MinSlot(300)).Should(BeEquivalentTo(300))
	g.Expect(Calculate5MinSlot(301)).Should(BeEquivalentTo(300))
	g.Expect(Calculate5MinSlot(599)).Should(BeEquivalentTo(300))
	g.Expect(Calculate5MinSlot(601)).Should(BeEquivalentTo(600))
}
