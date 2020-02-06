package hooks

import (
	"github.com/onsi/gomega/gbytes"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("Modules :: cloud-instance-manager :: hooks :: get_crds ::", func() {
	const (
		stateCIGProper = `
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: proper1
spec:
  instanceClassReference:
    kind: D8TestInstanceClass
    name: proper1
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: proper2
spec:
  instanceClassReference:
    kind: D8TestInstanceClass
    name: proper2
  zones: [a,b]

`
		stateCIGWrongKind = `
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: improper
spec:
  instanceClassReference:
    kind: ImproperInstanceClass
    name: improper
`
		stateCIGWrongName = `
---
apiVersion: deckhouse.io/v1alpha1
kind: CloudInstanceGroup
metadata:
  name: improper
spec:
  instanceClassReference:
    kind: D8TestInstanceClass
    name: improper
`
		stateICProper = `
---
apiVersion: deckhouse.io/v1alpha1
kind: D8TestInstanceClass
metadata:
  name: proper1
spec:
  cloudInitSteps:
    options:
      version: centos-7.1.1.1
      kubernetesVersion: 1.15.4
---
apiVersion: deckhouse.io/v1alpha1
kind: D8TestInstanceClass
metadata:
  name: proper2
spec:
  cloudInitSteps:
    options:
      version: slackware-14.1
`
		stateICIMroper = `
---
apiVersion: deckhouse.io/v1alpha1
kind: D8TestInstanceClass
metadata:
  name: improper1
spec:
  cloudInitSteps:
    options:
      version: ubuntu-7.1.1.1
`

		stateCloudProviderSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-cloud-instance-manager-cloud-provider
  namespace: kube-system
data:
  zones: WyJub3ZhIl0= # ["nova"]
`
	)

	f := HookExecutionConfigInit(`{"cloudInstanceManager":{"internal": {}}}`, `{}`)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "CloudInstanceGroup", false)
	f.RegisterCRD("deckhouse.io", "v1alpha1", "D8TestInstanceClass", false)

	Context("Empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(``))
			f.RunHook()
		})

		It("Hook must not fail", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Proper cluster with two pairs of CIG+IC and provider secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIGProper + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("CIGs must be stored to cloudInstanceManager.internal.instanceGroups", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
        [
          {
            "instanceClassReference": {
              "kind": "D8TestInstanceClass",
              "name": "proper1"
            },
            "name": "proper1",
            "instanceClass": {
              "cloudInitSteps": {
                "options": {
                  "kubernetesVersion": "1.15.4",
                  "version": "centos-7.1.1.1"
                }
              }
            },
            "zones": [
              "nova"
            ]
          },
          {
            "instanceClassReference": {
              "kind": "D8TestInstanceClass",
              "name": "proper2"
            },
            "zones": [
              "a",
              "b"
            ],
            "name": "proper2",
            "instanceClass": {
              "cloudInitSteps": {
                "options": {
                  "version": "slackware-14.1"
                }
              }
            }
          }
        ]
`
			Expect(f.ValuesGet("cloudInstanceManager.internal.instanceGroups").String()).To(MatchJSON(expectedJSON))
		})
	})

	Context("Cluster with two proper pairs of CIG+IC, one improper IC and provider secret", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIGProper + stateICProper + stateICIMroper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("CIGs must be stored to cloudInstanceManager.internal.instanceGroups", func() {
			Expect(f).To(ExecuteSuccessfully())

			expectedJSON := `
        [
          {
            "instanceClassReference": {
              "kind": "D8TestInstanceClass",
              "name": "proper1"
            },
            "name": "proper1",
            "instanceClass": {
              "cloudInitSteps": {
                "options": {
                  "kubernetesVersion": "1.15.4",
                  "version": "centos-7.1.1.1"
                }
              }
            },
            "zones": [
              "nova"
            ]
          },
          {
            "instanceClassReference": {
              "kind": "D8TestInstanceClass",
              "name": "proper2"
            },
            "zones": [
              "a",
              "b"
            ],
            "name": "proper2",
            "instanceClass": {
              "cloudInitSteps": {
                "options": {
                  "version": "slackware-14.1"
                }
              }
            }
          }
        ]
`
			Expect(f.ValuesGet("cloudInstanceManager.internal.instanceGroups").String()).To(MatchJSON(expectedJSON))
			Expect(f.Session.Err).Should(gbytes.Say("Instance class improper1 is invalid: .spec.cloudInitSteps.options.kubernetesVersion is mandatory for .spec.cloudInitSteps.version ubuntu-7.1.1.1"))
		})

	})

	Context("Two proper pairs of CIG+IC and a CIG with wrong ref kind", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIGProper + stateCIGWrongKind + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Hook must fail due to unsupported Kind if CIG", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
			Expect(f.Session.Err).Should(gbytes.Say("Bad instanceClassReference in CloudInstanceGroup improper: kind ImproperInstanceClass is not allowed, the only allowed kind is D8TestInstanceClass"))
		})
	})

	Context("Two proper pairs of CIG+IC and a CIG with wrong ref name", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(stateCIGProper + stateCIGWrongName + stateICProper + stateCloudProviderSecret))
			f.RunHook()
		})

		It("Hook must fail due to wrong reference to IC", func() {
			Expect(f).To(Not(ExecuteSuccessfully()))
			Expect(f.Session.Err).Should(gbytes.Say(`Bad instanceClassReference in CloudInstanceGroup "improper": there is no valid instance class "improper" of type "D8TestInstanceClass"`))
		})
	})

})
