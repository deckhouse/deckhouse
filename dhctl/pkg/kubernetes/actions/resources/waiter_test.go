package resources

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/deckhouse/deckhouse/dhctl/pkg/template"
)

func TestResourcesToCheckers(t *testing.T) {
	const resourcesContentWithoutNg = `
---
apiVersion: deckhouse.io/v1
kind: YandexInstanceClass
metadata:
  name: system
spec:
  cores: 4
  memory: 8192
---
apiVersion: deckhouse.io/v1
kind: ClusterAuthorizationRule
metadata:
  name: admin
spec:
  subjects:
  - kind: User
    name: admin@admin.yoyo
  accessLevel: SuperAdmin
  portForwarding: true
---
`

	t.Run("without nodegroup", func(t *testing.T) {
		resources, err := template.ParseResourcesContent(resourcesContentWithoutNg, nil)
		require.NoError(t, err)
		require.Len(t, resources, 2)

		checkers, err := GetCheckers(nil, resources)
		require.NoError(t, err)
		require.Len(t, checkers, 0)
	})

	t.Run("with cloud static nodegroup", func(t *testing.T) {
		const content = resourcesContentWithoutNg + `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: node
spec:
  nodeType: Static
`
		resources, err := template.ParseResourcesContent(content, nil)
		require.NoError(t, err)
		require.Len(t, resources, 3)

		checkers, err := GetCheckers(nil, resources)
		require.NoError(t, err)
		require.Len(t, checkers, 0, "should skip")
	})

	t.Run("with cloud ephemeral nodegroup, but min and max per zone not set", func(t *testing.T) {
		const content = resourcesContentWithoutNg + `
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: system
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: system
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: system
  nodeType: CloudEphemeral
`
		resources, err := template.ParseResourcesContent(content, nil)
		require.NoError(t, err)
		require.Len(t, resources, 3)

		checkers, err := GetCheckers(nil, resources)
		require.NoError(t, err)
		require.Len(t, checkers, 0, "should skip")
	})

	ngTemplate := func(name string, min, max int) string {
		return fmt.Sprintf(`
apiVersion: deckhouse.io/v1
kind: NodeGroup
metadata:
  name: %s
spec:
  cloudInstances:
    classReference:
      kind: YandexInstanceClass
      name: system
    minPerZone: %d
    maxPerZone: %d
  nodeTemplate:
    labels:
      node-role.deckhouse.io/system: ""
    taints:
      - effect: NoExecute
        key: dedicated.deckhouse.io
        value: system
  nodeType: CloudEphemeral
---
`, name, min, max)
	}

	t.Run("with cloud ephemeral nodegroup, but min and max per zone is zero", func(t *testing.T) {
		content := resourcesContentWithoutNg + ngTemplate("system", 0, 0)

		resources, err := template.ParseResourcesContent(content, nil)
		require.NoError(t, err)
		require.Len(t, resources, 3)

		checkers, err := GetCheckers(nil, resources)
		require.NoError(t, err)
		require.Len(t, checkers, 0, "should skip")
	})

	t.Run("with cloud ephemeral nodegroup, but min = 0 and max not zero", func(t *testing.T) {
		content := resourcesContentWithoutNg + ngTemplate("system", 0, 2)

		resources, err := template.ParseResourcesContent(content, nil)
		require.NoError(t, err)
		require.Len(t, resources, 3)

		checkers, err := GetCheckers(nil, resources)
		require.NoError(t, err)
		require.Len(t, checkers, 1, "should get check")

		require.Equal(t, checkers[0].Name(), "NodeGroup system readiness check")
	})

	t.Run("with cloud ephemeral nodegroup, but min not zero and max not zero", func(t *testing.T) {
		content := resourcesContentWithoutNg + ngTemplate("system", 1, 2)

		resources, err := template.ParseResourcesContent(content, nil)
		require.NoError(t, err)
		require.Len(t, resources, 3)

		checkers, err := GetCheckers(nil, resources)
		require.NoError(t, err)
		require.Len(t, checkers, 1, "should get check")

		require.Equal(t, checkers[0].Name(), "NodeGroup system readiness check")
	})

	t.Run("with multiple cloud ephemeral nodegroup", func(t *testing.T) {
		content := resourcesContentWithoutNg +
			ngTemplate("system", 0, 2) +
			ngTemplate("node", 1, 2)

		resources, err := template.ParseResourcesContent(content, nil)
		require.NoError(t, err)
		require.Len(t, resources, 4)

		checkers, err := GetCheckers(nil, resources)
		require.NoError(t, err)
		require.Len(t, checkers, 2, "should get all check")

		require.Equal(t, checkers[0].Name(), "NodeGroup system readiness check")
		require.Equal(t, checkers[1].Name(), "NodeGroup node readiness check")
	})
}
