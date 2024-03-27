package requirements

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestModuleRegexp(t *testing.T) {
	funcName := "github.com/deckhouse/deckhouse/modules/402-ingress-nginx/requirements.init.0.func1"
	rr := mreg.FindStringSubmatch(funcName)
	assert.Equal(t, "ingress-nginx", rr[1])
}
