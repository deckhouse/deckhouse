/*
Copyright 2021 Flant JSC
Licensed under the Deckhouse Platform Enterprise Edition (EE) license. See https://github.com/deckhouse/deckhouse/blob/main/ee/LICENSE
*/

package hooks

import (
	"os"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)

	opts := git.CloneOptions{
		URL:           "https://github.com/flant/shell-operator",
		ReferenceName: plumbing.NewTagReferenceName("v1.1.3"),
		SingleBranch:  true,
		Tags:          git.NoTags,
		Depth:         1,
	}
	_, err := git.PlainClone("/deckhouse/shell-operator", false, &opts)
	if err != nil {
		panic(err)
	}

	defer func() {
		_ = os.RemoveAll("/deckhouse/shell-operator")
	}()

	RunSpecs(t, "")
}
