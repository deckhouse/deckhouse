package util

import (
	"fmt"
	"testing"
)

func Test_RndAlphaNum(t *testing.T) {
	t.SkipNow()
	fmt.Println(RndAlphaNum(5))
}

func Test_RandomIdentifier(t *testing.T) {
	t.SkipNow()
	fmt.Println(RandomIdentifier("upmeter-test-object"))
}
