package entity

import (
	"testing"

	. "github.com/onsi/gomega"
)

func Test_CalculateAdjustedStepRanges(t *testing.T) {
	g := NewWithT(t)

	steps := CalculateAdjustedStepRanges(0, 300, 300)
	g.Expect(steps.Ranges).Should(HaveLen(1))
	g.Expect(steps.Ranges[0][0]).Should(BeEquivalentTo(0))
	g.Expect(steps.Ranges[0][1]).Should(BeEquivalentTo(300))
	//g.Expect(steps.Ranges[1][0]).Should(BeEquivalentTo(300))
	//g.Expect(steps.Ranges[1][1]).Should(BeEquivalentTo(600))

	// Adjusts
	steps = CalculateAdjustedStepRanges(21, 663, 321)
	g.Expect(steps.Ranges).Should(HaveLen(2))
	g.Expect(steps.Ranges[0][0]).Should(BeEquivalentTo(300))
	g.Expect(steps.Ranges[0][1]).Should(BeEquivalentTo(600))
	g.Expect(steps.Ranges[1][0]).Should(BeEquivalentTo(600))
	g.Expect(steps.Ranges[1][1]).Should(BeEquivalentTo(900))
	//g.Expect(steps.Ranges[2][0]).Should(BeEquivalentTo(600))
	//g.Expect(steps.Ranges[2][1]).Should(BeEquivalentTo(900))

	// Bigger step (1h)
	steps = CalculateAdjustedStepRanges(21, 10000, 3600)
	g.Expect(steps.Ranges).Should(HaveLen(2))
	//g.Expect(steps.Ranges[0][0]).Should(BeEquivalentTo(0))
	//g.Expect(steps.Ranges[0][1]).Should(BeEquivalentTo(3600))
	g.Expect(steps.Ranges[0][0]).Should(BeEquivalentTo(3600))
	g.Expect(steps.Ranges[0][1]).Should(BeEquivalentTo(7200))
	g.Expect(steps.Ranges[1][0]).Should(BeEquivalentTo(7200))
	g.Expect(steps.Ranges[1][1]).Should(BeEquivalentTo(10800))

	// Step ranges used in status_test.go
	steps = CalculateAdjustedStepRanges(0, 900, 300)
	g.Expect(steps.Ranges).Should(HaveLen(3))
	g.Expect(steps.Ranges[0][0]).Should(BeEquivalentTo(0))
	g.Expect(steps.Ranges[0][1]).Should(BeEquivalentTo(300))
	g.Expect(steps.Ranges[1][0]).Should(BeEquivalentTo(300))
	g.Expect(steps.Ranges[1][1]).Should(BeEquivalentTo(600))
	g.Expect(steps.Ranges[2][0]).Should(BeEquivalentTo(600))
	g.Expect(steps.Ranges[2][1]).Should(BeEquivalentTo(900))
	// Step ranges used in status_test.go
	steps = CalculateAdjustedStepRanges(0, 1200, 600)
	g.Expect(steps.Ranges).Should(HaveLen(2))
	g.Expect(steps.Ranges[0][0]).Should(BeEquivalentTo(0))
	g.Expect(steps.Ranges[0][1]).Should(BeEquivalentTo(600))
	g.Expect(steps.Ranges[1][0]).Should(BeEquivalentTo(600))
	g.Expect(steps.Ranges[1][1]).Should(BeEquivalentTo(1200))

	// Big step
	steps = CalculateAdjustedStepRanges(10000, 70000, 7200)
	g.Expect(steps.Ranges).Should(HaveLen((70000 - 10000) / 7200))
	step := steps.Ranges[0]
	g.Expect(step[0]).Should(BeEquivalentTo(14400))
	g.Expect(step[1]).Should(BeEquivalentTo(14400 + 7200))
	step = steps.Ranges[len(steps.Ranges)-1]
	g.Expect(step[0]).Should(BeEquivalentTo(72000 - 7200))
	g.Expect(step[1]).Should(BeEquivalentTo(72000))

	// Real timestamps
	steps = CalculateAdjustedStepRanges(1603180029, 1603784829, 86400)
	g.Expect(steps.From).Should(BeEquivalentTo(1603238400))
	g.Expect(steps.To).Should(BeEquivalentTo(1603843200))
	g.Expect(steps.Ranges).Should(HaveLen(7))
	//step := steps.Ranges[0]
	//g.Expect(step[0]).Should(BeEquivalentTo(7200))
	//g.Expect(step[1]).Should(BeEquivalentTo(7200 * 2))
	//step = steps.Ranges[len(steps.Ranges)-1]
	//g.Expect(step[0]).Should(BeEquivalentTo(72000 - 7200))
	//g.Expect(step[1]).Should(BeEquivalentTo(72000))
}
