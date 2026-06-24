package markers

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestMarkers(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Markers Suite")
}
