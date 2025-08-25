// Copyright 2024 Flant JSC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package release

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	addonmodules "github.com/flant/addon-operator/pkg/module_manager/models/modules"
	addonutils "github.com/flant/addon-operator/pkg/utils"
	metricstorage "github.com/flant/shell-operator/pkg/metric_storage"
	crv1 "github.com/google/go-containerregistry/pkg/v1"
	crfake "github.com/google/go-containerregistry/pkg/v1/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"helm.sh/helm/v3/pkg/releaseutil"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	validationerrors "k8s.io/kube-openapi/pkg/validation/errors"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha1"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/apis/deckhouse.io/v1alpha2"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/module-controllers/utils"
	moduletypes "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/controller/moduleloader/types"
	d8edition "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/edition"
	"github.com/deckhouse/deckhouse/deckhouse-controller/pkg/helpers"
	releaseUpdater "github.com/deckhouse/deckhouse/deckhouse-controller/pkg/releaseupdater"
	"github.com/deckhouse/deckhouse/go_lib/d8env"
	"github.com/deckhouse/deckhouse/go_lib/dependency"
	"github.com/deckhouse/deckhouse/go_lib/dependency/extenders"
	"github.com/deckhouse/deckhouse/go_lib/hooks/update"
	"github.com/deckhouse/deckhouse/pkg/log"
	"github.com/deckhouse/deckhouse/testing/controller/controllersuite"
)

var (
	mDelimiter = regexp.MustCompile("(?m)^---$")

	embeddedMUP = &v1alpha2.ModuleUpdatePolicySpec{
		Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
			Mode:    v1alpha2.UpdateModeAuto.String(),
			Windows: make(update.Windows, 0),
		},
		ReleaseChannel: "Stable",
	}
	golden bool
)

func init() {
	flag.BoolVar(&golden, "golden", false, "generate golden files")
}

func TestReleaseControllerTestSuite(t *testing.T) {
	suite.Run(t, new(ReleaseControllerTestSuite))
}

type ReleaseControllerTestSuite struct {
	controllersuite.Suite

	client client.Client
	ctr    *reconciler

	testDataFileName string
	testMRName       string
}

func (suite *ReleaseControllerTestSuite) SetupSubTest() {
	suite.Suite.SetupSubTest()

	suite.T().Setenv(d8env.DownloadedModulesDir, suite.TmpDir())
	moduleDir := filepath.Join(suite.TmpDir(), "modules")
	err := os.MkdirAll(moduleDir, 0o777)
	if errors.Is(err, os.ErrExist) {
		err = nil
	}
	suite.Check(err)
}

func (suite *ReleaseControllerTestSuite) TearDownSubTest() {
	defer suite.Suite.TearDownSubTest()

	if suite.T().Skipped() || suite.T().Failed() {
		return
	}

	goldenFile := filepath.Join("./testdata/releases", "golden", suite.testDataFileName)
	gotB := suite.fetchResults()

	if golden {
		err := os.WriteFile(goldenFile, gotB, 0o666)
		require.NoError(suite.T(), err)
	} else {
		got := singleDocToManifests(gotB)
		expB, err := os.ReadFile(goldenFile)
		require.NoError(suite.T(), err)
		exp := singleDocToManifests(expB)
		assert.Equal(suite.T(), len(got), len(exp), "The number of `got` manifests must be equal to the number of `exp` manifests")
		for i := range got {
			assert.YAMLEq(suite.T(), exp[i], got[i], "Got and exp manifests must match")
		}
	}
}

func (suite *ReleaseControllerTestSuite) TestCreateReconcile() {
	err := os.Setenv("TEST_EXTENDER_DECKHOUSE_VERSION", "v1.0.0")
	require.NoError(suite.T(), err)
	err = os.Setenv("TEST_EXTENDER_KUBERNETES_VERSION", "1.28.0")
	require.NoError(suite.T(), err)
	ctx := suite.Context()

	dependency.TestDC.CRClient.ImageMock.Return(&crfake.FakeImage{
		ManifestStub: func() (*crv1.Manifest, error) {
			return &crv1.Manifest{
				Layers: []crv1.Descriptor{},
			}, nil
		},
		LayersStub: func() ([]crv1.Layer, error) {
			return []crv1.Layer{&utils.FakeLayer{}}, nil
		},
	}, nil)

	suite.Run("simple", func() {
		suite.setupReleaseController(suite.fetchTestFileData("simple.yaml"))

		repeatTest(func() {
			mr := suite.getModuleRelease(suite.testMRName)
			_, err = suite.ctr.handleRelease(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})
	})

	suite.Run("with annotation", func() {
		suite.setupReleaseController(suite.fetchTestFileData("with-annotation.yaml"))

		repeatTest(func() {
			mr := suite.getModuleRelease(suite.testMRName)
			_, err = suite.ctr.handleRelease(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})
	})

	suite.Run("deckhouse suitable version", func() {
		suite.setupReleaseController(suite.fetchTestFileData("dVersion-suitable.yaml"))

		repeatTest(func() {
			mr := suite.getModuleRelease(suite.testMRName)
			_, err = suite.ctr.handleRelease(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})
	})

	suite.Run("deckhouse unsuitable version", func() {
		suite.setupReleaseController(suite.fetchTestFileData("dVersion-unsuitable.yaml"))

		repeatTest(func() {
			mr := suite.getModuleRelease(suite.testMRName)
			_, err = suite.ctr.handleRelease(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})
	})

	suite.Run("kubernetes suitable version", func() {
		suite.setupReleaseController(suite.fetchTestFileData("kVersion-suitable.yaml"))

		repeatTest(func() {
			mr := suite.getModuleRelease(suite.testMRName)
			_, err = suite.ctr.handleRelease(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})
	})

	suite.Run("kubernetes unsuitable version", func() {
		suite.setupReleaseController(suite.fetchTestFileData("kVersion-unsuitable.yaml"))

		repeatTest(func() {
			mr := suite.getModuleRelease(suite.testMRName)
			_, err = suite.ctr.handleRelease(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})
	})

	suite.Run("deploy with outdated module releases", func() {
		dependency.TestDC.CRClient.ListTagsMock.Return([]string{}, nil)
		suite.setupReleaseController(suite.fetchTestFileData("clean-up-outdated-module-releases-when-deploy.yaml"))
		suite.updateModuleReleasesStatuses()

		repeatTest(func() {
			_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("echo-v0.4.54"))
			require.NoError(suite.T(), err)
		})
	})

	suite.Run("clean up for a deployed module release with outdated module releases", func() {
		dependency.TestDC.CRClient.ListTagsMock.Return([]string{}, nil)
		suite.setupReleaseController(suite.fetchTestFileData("clean-up-outdated-module-releases-for-deployed.yaml"))
		suite.updateModuleReleasesStatuses()

		repeatTest(func() {
			_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("echo-v0.4.54"))
			require.NoError(suite.T(), err)
		})
	})

	suite.Run("loop until deploy: canary", func() {
		dc := dependency.NewMockedContainer()
		dc.CRClient.ImageMock.Return(&crfake.FakeImage{LayersStub: func() ([]crv1.Layer, error) {
			return []crv1.Layer{&utils.FakeLayer{}}, nil
		}}, nil)

		mup := &v1alpha2.ModuleUpdatePolicySpec{
			Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
				Mode:    "Auto",
				Windows: update.Windows{{From: update.MinTime, To: update.MaxTime, Days: []string{"Thu"}}},
			},
			ReleaseChannel: "Stable",
		}

		testData := suite.fetchTestFileData("loop-canary.yaml")
		suite.setupReleaseController(testData, withModuleUpdatePolicy(mup), withDependencyContainer(dc))

		repeatTest(func() {
			suite.loopUntilDeploy(dc, suite.testMRName)
		})
	})

	suite.Run("install new module in manual mode with deckhouse release approval annotation", func() {
		suite.setupReleaseController(suite.fetchTestFileData("new-module-manual-mode.yaml"))

		repeatTest(func() {
			_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease(suite.testMRName))
			require.NoError(suite.T(), err)
		})
	})

	suite.Run("AutoPatch", func() {
		suite.Run("patch update respect window", func() {
			mup := &v1alpha2.ModuleUpdatePolicySpec{
				Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
					Mode:    "AutoPatch",
					Windows: update.Windows{{From: "10:00", To: "11:00", Days: update.Everyday()}},
				},
				ReleaseChannel: "Stable",
			}

			testData := suite.fetchTestFileData("auto-patch-patch-update.yaml")
			suite.setupReleaseController(testData, withModuleUpdatePolicy(mup))

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.26.2"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.26.3"))
				require.NoError(suite.T(), err)
			})
		})

		suite.Run("minor update don't respect window", func() {
			mup := &v1alpha2.ModuleUpdatePolicySpec{
				Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
					Mode:    "AutoPatch",
					Windows: update.Windows{{From: "10:00", To: "11:00", Days: update.Everyday()}},
				},
				ReleaseChannel: "Stable",
			}

			testData := suite.fetchTestFileData("auto-patch-minor-update.yaml")
			suite.setupReleaseController(testData, withModuleUpdatePolicy(mup))

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.26.2"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.27.0"))
				require.NoError(suite.T(), err)
			})
		})

		suite.Run("Postponed release", func() {
			mup := &v1alpha2.ModuleUpdatePolicySpec{
				Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
					Mode:    "AutoPatch",
					Windows: update.Windows{{From: "10:00", To: "11:00", Days: update.Everyday()}},
				},
				ReleaseChannel: "Stable",
			}

			testData := suite.fetchTestFileData("auto-mode.yaml")
			suite.setupReleaseController(testData, withModuleUpdatePolicy(mup))

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.26.2"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.27.0"))
				require.NoError(suite.T(), err)
			})
		})

		suite.Run("Postponed patch release", func() {
			mup := embeddedMUP.DeepCopy()
			mup.Update.Mode = v1alpha2.UpdateModeAutoPatch.String()

			testData := suite.fetchTestFileData("auto-patch-mode.yaml")
			suite.setupReleaseController(testData, withModuleUpdatePolicy(mup))

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.26.2"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.26.3"))
				require.NoError(suite.T(), err)
			})
		})

		suite.Run("Postponed minor release", func() {
			mup := embeddedMUP.DeepCopy()
			mup.Update.Mode = v1alpha2.UpdateModeAutoPatch.String()

			testData := suite.fetchTestFileData("auto-patch-mode-minor-release.yaml")
			suite.setupReleaseController(testData, withModuleUpdatePolicy(mup))

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.26.2"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.27.0"))
				require.NoError(suite.T(), err)
			})
		})

		suite.Run("Approved minor release", func() {
			mup := embeddedMUP.DeepCopy()
			mup.Update.Mode = v1alpha2.UpdateModeAutoPatch.String()

			testData := suite.fetchTestFileData("auto-patch-mode-minor-release-approved.yaml")
			suite.setupReleaseController(testData, withModuleUpdatePolicy(mup))

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.26.2"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.27.0"))
				require.NoError(suite.T(), err)
			})
		})
	})

	suite.Run("Patch awaits update window", func() {
		mup := embeddedMUP.DeepCopy()
		mup.Update.Windows = update.Windows{{From: "8:00", To: "8:01", Days: update.Everyday()}}

		testData := suite.fetchTestFileData("patch-awaits-update-window.yaml")
		suite.setupReleaseController(testData, withModuleUpdatePolicy(mup))

		repeatTest(func() {
			_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.26.2"))
			require.NoError(suite.T(), err)
			_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.26.3"))
			require.NoError(suite.T(), err)
		})
	})

	suite.Run("Reinstall", func() {
		mup := &v1alpha2.ModuleUpdatePolicySpec{
			Update: v1alpha2.ModuleUpdatePolicySpecUpdate{
				Mode:    "AutoPatch",
				Windows: update.Windows{{From: "10:00", To: "11:00", Days: update.Everyday()}},
			},
			ReleaseChannel: "Stable",
		}

		testData := suite.fetchTestFileData("reinstall-annotation.yaml")
		suite.setupReleaseController(testData, withModuleUpdatePolicy(mup))

		repeatTest(func() {
			_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease("parca-1.26.2"))
			require.NoError(suite.T(), err)
			require.NoError(suite.T(), err)
		})
	})

	suite.Run("Process force release", func() {
		suite.setupReleaseController(suite.fetchTestFileData("apply-force-release.yaml"))

		repeatTest(func() {
			mr := suite.getModuleRelease("parca-1.2.1")
			_, err := suite.ctr.handleRelease(context.TODO(), mr)
			require.NoError(suite.T(), err)

			mr = suite.getModuleRelease("parca-1.5.2")
			_, err = suite.ctr.handleRelease(context.TODO(), mr)
			require.NoError(suite.T(), err)
		})
	})

	suite.Run("Sequential processing", func() {
		suite.Run("sequential processing with patch release", func() {
			testData := suite.fetchTestFileData("sequential-processing-patch.yaml")
			suite.setupReleaseController(testData)

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.70.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.70.1"))
				require.NoError(suite.T(), err)
			})
		})

		suite.Run("sequential processing with minor release", func() {
			testData := suite.fetchTestFileData("sequential-processing-minor.yaml")
			suite.setupReleaseController(testData)

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.70.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.71.0"))
				require.NoError(suite.T(), err)
			})
		})

		suite.Run("sequential processing with minor pending release", func() {
			testData := suite.fetchTestFileData("sequential-processing-minor-pending.yaml")
			suite.setupReleaseController(testData)

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.70.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.71.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.72.0"))
				require.NoError(suite.T(), err)
			})
		})

		suite.Run("sequential processing with minor auto release", func() {
			testData := suite.fetchTestFileData("sequential-processing-minor-auto.yaml")
			suite.setupReleaseController(testData)

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.70.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.71.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.72.0"))
				require.NoError(suite.T(), err)
			})
		})

		suite.Run("sequential processing with minor notready release", func() {
			testData := suite.fetchTestFileData("sequential-processing-minor-notready.yaml")
			suite.setupReleaseController(testData, withBasicModulePhase(addonmodules.Startup))

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.70.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.71.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.72.0"))
				require.NoError(suite.T(), err)
			})
		})

		suite.Run("sequential processing with pending releases", func() {
			testData := suite.fetchTestFileData("sequential-processing-pending.yaml")
			suite.setupReleaseController(testData, withBasicModulePhase(addonmodules.Startup))

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.70.0"))
				require.NoError(suite.T(), err)
				suite.setModulePhase(addonmodules.Ready)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.71.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("upmeter-v1.72.0"))
				require.NoError(suite.T(), err)
			})
		})
	})

	suite.Run("Process pending releases", func() {
		// Setup initial state
		suite.setupReleaseController(suite.fetchTestFileData("apply-pending-releases.yaml"))

		repeatTest(func() {
			// Test updating Parca module
			mr := suite.getModuleRelease("parca-1.2.2")
			_, err := suite.ctr.handleRelease(ctx, mr)
			require.NoError(suite.T(), err)

			// Test updating Commander module
			mr = suite.getModuleRelease("commander-1.0.3")
			_, err = suite.ctr.handleRelease(ctx, mr)
			require.NoError(suite.T(), err)

			// Verify the final state
			parca := suite.getModuleRelease("parca-1.2.2")
			require.Equal(suite.T(), v1alpha1.ModuleReleasePhaseDeployed, parca.Status.Phase)

			commander := suite.getModuleRelease("commander-1.0.3")
			require.Equal(suite.T(), v1alpha1.ModuleReleasePhaseDeployed, commander.Status.Phase)
		})
	})

	suite.Run("Process major releases", func() {
		suite.Run("major release from 0 to 1 must be allowed", func() {
			// Setup initial state
			suite.setupReleaseController(suite.fetchTestFileData("update-major-version-0-1.yaml"))

			repeatTest(func() {
				mr := suite.getModuleRelease("parca-0.26.2")
				_, err := suite.ctr.handleRelease(ctx, mr)
				require.NoError(suite.T(), err)

				// Test updating Parca module
				mr = suite.getModuleRelease("parca-1.0.0")
				_, err = suite.ctr.handleRelease(ctx, mr)
				require.NoError(suite.T(), err)
			})
		})

		suite.Run("major release from 1 to 2 must be not allowed", func() {
			// Setup initial state
			suite.setupReleaseController(suite.fetchTestFileData("update-major-version-1-2.yaml"))

			repeatTest(func() {
				mr := suite.getModuleRelease("parca-1.26.2")
				_, err := suite.ctr.handleRelease(ctx, mr)
				require.NoError(suite.T(), err)

				// Test updating Parca module
				mr = suite.getModuleRelease("parca-2.0.0")
				_, err = suite.ctr.handleRelease(ctx, mr)
				require.NoError(suite.T(), err)
			})
		})
	})
}

func (suite *ReleaseControllerTestSuite) loopUntilDeploy(dc *dependency.MockedContainer, releaseName string) {
	const maxIterations = 3
	suite.T().Skip("TODO: requeue all releases after got deckhouse module config update")

	var (
		result = ctrl.Result{Requeue: true}
		err    error
		i      int
	)

	// Setting needsRestart field in real code causes the process to reboot.
	// And at the next startup, Reconcile will be called for existing objects.
	// Therefore, this condition emulates the behavior in real code.
	for result.Requeue || result.RequeueAfter > 0 || atomic.LoadInt64(&suite.ctr.needsRestart) != 0 {
		atomic.StoreInt64(&suite.ctr.needsRestart, 0)
		dc.GetFakeClock().Advance(result.RequeueAfter)

		dr := suite.getModuleRelease(releaseName)
		if dr.Status.Phase == v1alpha1.ModuleReleasePhaseDeployed {
			return
		}

		result, err = suite.ctr.handleRelease(context.TODO(), dr)
		require.NoError(suite.T(), err)

		i++
		if i > maxIterations {
			suite.T().Fatal("Too many iterations")
		}
		suite.ctr.log.Info("Iteration result:", slog.Int("iteration", i), slog.Any("result", result))
	}

	suite.T().Fatal("Loop was broken")
}

func (suite *ReleaseControllerTestSuite) updateModuleReleasesStatuses() {
	releases := new(v1alpha1.ModuleReleaseList)
	require.NoError(suite.T(), suite.client.List(context.TODO(), releases))

	caser := cases.Title(language.English)
	for _, release := range releases.Items {
		release.Status.Phase = caser.String(release.Labels[v1alpha1.ModuleReleaseLabelStatus])
		require.NoError(suite.T(), suite.client.Status().Update(context.TODO(), &release))
	}
}

func (suite *ReleaseControllerTestSuite) setModulePhase(phase addonmodules.ModuleRunPhase) {
	suite.ctr.moduleManager = stubModulesManager{
		modulePhase: phase,
	}
}

type reconcilerOption func(*reconciler)

func withModuleUpdatePolicy(mup *v1alpha2.ModuleUpdatePolicySpec) reconcilerOption {
	return func(r *reconciler) {
		r.embeddedPolicy = helpers.NewModuleUpdatePolicySpecContainer(mup)
	}
}

func withDependencyContainer(dc dependency.Container) reconcilerOption {
	return func(r *reconciler) {
		r.dependencyContainer = dc
	}
}

func withBasicModulePhase(phase addonmodules.ModuleRunPhase) reconcilerOption {
	return func(r *reconciler) {
		r.moduleManager = stubModulesManager{modulePhase: phase}
	}
}

func (suite *ReleaseControllerTestSuite) setupReleaseController(yamlDoc string, options ...reconcilerOption) {
	manifests := releaseutil.SplitManifests(yamlDoc)

	manifests["deckhouse-discovery"] = `
---
apiVersion: v1
data:
  bundle: RGVmYXVsdA==
  releaseChannel: VW5rbm93bg==
  updateSettings.json: eyJkaXNydXB0aW9uQXBwcm92YWxNb2RlIjoiQXV0byIsIm1vZGUiOiJNYW51YWwifQ==
kind: Secret
metadata:
  annotations:
    meta.helm.sh/release-name: deckhouse
    meta.helm.sh/release-namespace: d8-system
  creationTimestamp: "2024-01-18T14:29:03Z"
  labels:
    app.kubernetes.io/managed-by: Helm
    heritage: deckhouse
    module: deckhouse
  name: deckhouse-discovery
  namespace: d8-system
  resourceVersion: "134952280"
  uid: 7016bec6-b17c-4e90-bd35-16456d0df532
type: Opaque
`

	initObjects := make([]client.Object, 0, len(manifests))
	for _, manifest := range manifests {
		obj := suite.assembleInitObject(manifest)
		initObjects = append(initObjects, obj)
	}

	err := suite.Suite.SetupNoLock(initObjects)
	require.NoError(suite.T(), err)
	logger := log.NewNop()

	rec := &reconciler{
		client:               suite.Suite.Client(),
		downloadedModulesDir: d8env.GetDownloadedModulesDir(),
		dependencyContainer:  dependency.NewDependencyContainer(),
		log:                  logger,
		symlinksDir:          filepath.Join(d8env.GetDownloadedModulesDir(), "modules"),
		moduleManager:        stubModulesManager{},
		delayTimer:           time.NewTimer(3 * time.Second),
		metricStorage:        metricstorage.NewMetricStorage(context.Background(), "", true, logger),

		embeddedPolicy: helpers.NewModuleUpdatePolicySpecContainer(embeddedMUP),
		metricsUpdater: releaseUpdater.NewMetricsUpdater(metricstorage.NewMetricStorage(context.Background(), "", true, logger), releaseUpdater.ModuleReleaseBlockedMetricName),
		exts:           extenders.NewExtendersStack(new(d8edition.Edition), nil, log.NewNop()),
	}

	for _, option := range options {
		option(rec)
	}

	c := suite.Client()
	mup := &v1alpha2.ModuleUpdatePolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       v1alpha2.ModuleUpdatePolicyGVK.Kind,
			APIVersion: v1alpha2.ModuleUpdatePolicyGVK.GroupVersion().String(),
		},
		Spec: ptr.Deref(rec.embeddedPolicy.Get(), v1alpha2.ModuleUpdatePolicySpec{}),
	}
	result := c.Validator().Validate(mup)
	if result != nil {
		for _, warn := range skipNotSpecErrors(result.Warnings) {
			suite.Logger().Warn(warn.Error())
		}

		result.Errors = skipNotSpecErrors(result.Errors)
		if len(result.Errors) > 0 {
			suite.Check(fmt.Errorf("custom resource validation: %w", errors.Join(result.Errors...)))
		}
	}

	suite.ctr = rec
	suite.client = c
}

func skipNotSpecErrors(errs []error) []error {
	result := make([]error, 0, len(errs))
	for _, err := range errs {
		var vErr *validationerrors.Validation
		ok := errors.As(err, &vErr)
		if !ok || !strings.HasPrefix(vErr.Name, "spec.") {
			continue
		}

		result = append(result, err)
	}

	return result
}

func (suite *ReleaseControllerTestSuite) assembleInitObject(strObj string) client.Object {
	raw := []byte(strObj)

	metaType := new(runtime.TypeMeta)
	err := yaml.Unmarshal(raw, metaType)
	require.NoError(suite.T(), err)

	var obj client.Object

	switch metaType.Kind {
	case v1alpha1.ModuleSourceGVK.Kind:
		source := new(v1alpha1.ModuleSource)
		err = yaml.Unmarshal(raw, source)
		require.NoError(suite.T(), err)
		obj = source

	case v1alpha1.ModuleReleaseGVK.Kind:
		release := new(v1alpha1.ModuleRelease)
		err = yaml.Unmarshal(raw, release)
		require.NoError(suite.T(), err)
		obj = release
		suite.testMRName = release.Name

	case v1alpha2.ModuleUpdatePolicyGVK.Kind:
		policy := new(v1alpha2.ModuleUpdatePolicy)
		err = yaml.Unmarshal(raw, policy)
		require.NoError(suite.T(), err)
		obj = policy

	case v1alpha1.ModuleGVK.Kind:
		module := new(v1alpha1.Module)
		err = yaml.Unmarshal(raw, module)
		require.NoError(suite.T(), err)
		obj = module

	case "Secret":
		secret := new(corev1.Secret)
		err = yaml.Unmarshal(raw, secret)
		require.NoError(suite.T(), err)
		obj = secret
	}

	return obj
}

func (suite *ReleaseControllerTestSuite) fetchTestFileData(filename string) string {
	dir := "./testdata/releases"
	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(suite.T(), err)

	suite.testDataFileName = filename

	return string(data)
}

func (suite *ReleaseControllerTestSuite) getModuleRelease(name string) *v1alpha1.ModuleRelease {
	release := new(v1alpha1.ModuleRelease)
	err := suite.client.Get(context.TODO(), client.ObjectKey{Name: name}, release)
	require.NoError(suite.T(), err)

	return release
}

func (suite *ReleaseControllerTestSuite) fetchResults() []byte {
	result := bytes.NewBuffer(nil)

	sources := new(v1alpha1.ModuleSourceList)
	require.NoError(suite.T(), suite.client.List(suite.Context(), sources))

	for _, source := range sources.Items {
		got, _ := yaml.Marshal(source)
		result.WriteString("---\n")
		result.Write(got)
	}

	releases := new(v1alpha1.ModuleReleaseList)
	require.NoError(suite.T(), suite.client.List(context.TODO(), releases))

	for _, release := range releases.Items {
		got, _ := yaml.Marshal(release)
		result.WriteString("---\n")
		result.Write(got)
	}

	modules := new(v1alpha1.ModuleList)
	require.NoError(suite.T(), suite.client.List(context.TODO(), modules))

	for _, module := range modules.Items {
		got, _ := yaml.Marshal(module)
		result.WriteString("---\n")
		result.Write(got)
	}

	return result.Bytes()
}

type stubModulesManager struct {
	modulePhase addonmodules.ModuleRunPhase
}

func (s stubModulesManager) AreModulesInited() bool {
	return true
}

func (s stubModulesManager) DisableModuleHooks(_ string) {
}

func (s stubModulesManager) GetModule(name string) *addonmodules.BasicModule {
	bm, _ := addonmodules.NewBasicModule(name, "", 900, nil, []byte{}, []byte{}, addonmodules.WithLogger(log.NewNop()))
	bm.SetPhase(addonmodules.Ready)
	if s.modulePhase != "" {
		bm.SetPhase(s.modulePhase)
	}

	return bm
}

func (s stubModulesManager) GetEnabledModuleNames() []string {
	return nil
}

func (s stubModulesManager) IsModuleEnabled(_ string) bool {
	return true
}

func (s stubModulesManager) RunModuleWithNewOpenAPISchema(_, _, _ string) error {
	return nil
}

func singleDocToManifests(doc []byte) []string {
	split := mDelimiter.Split(string(doc), -1)

	result := make([]string, 0, len(split))
	for i := range split {
		if split[i] != "" {
			result = append(result, split[i])
		}
	}

	return result
}

func TestValidateModule(t *testing.T) {
	check := func(name string, failed bool, values addonutils.Values) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			def := moduletypes.Definition{
				Name:   name,
				Weight: 900,
				Path:   filepath.Join("./testdata", name),
			}

			err := def.Validate(values, log.NewNop())
			if !failed {
				require.NoError(t, err, "%s: unexpected error: %v", name, err)
			}

			if failed {
				require.Error(t, err, "%s: got nil error", name)
			}
		})
	}

	check("validation/module", false, nil)
	check("validation/module-not-valid", true, nil)
	check("validation/module-failed", true, nil)
	check("validation/module-values-failed", true, nil)
	check("validation/virtualization", false, addonutils.Values{
		"virtualMachineCIDRs": []any{},
		"dvcr": map[string]any{
			"storage": map[string]any{
				"persistentVolumeClaim": map[string]any{
					"size": "50G",
				},

				"type": "PersistentVolumeClaim",
			},
		},
	})
	check("validation/virtualization", true, nil)
}

func TestConcurrentModuleRestartFlow(t *testing.T) {
	t.Run("real world scenario: 3 concurrent modules wait for all to complete before restart", func(t *testing.T) {
		rec := &reconciler{
			log:                log.NewNop(),
			activeApplyCount:   0,
			hasAppliedReleases: 0,
			needsRestart:       0,
			delayTimer:         time.NewTimer(time.Second),
		}

		const numModules = 3
		var wg sync.WaitGroup
		moduleStarted := make(chan struct{}, numModules)
		allowCompletion := make(chan struct{})
		restartTriggered := int64(0)

		// Mock the restart condition check function
		checkRestartCondition := func() bool {
			return atomic.LoadInt64(&rec.needsRestart) == 1 &&
				atomic.LoadInt64(&rec.activeApplyCount) == 0 &&
				atomic.LoadInt64(&rec.hasAppliedReleases) == 1
		}

		// Simulate restart checker (like restartLoop)
		go func() {
			ticker := time.NewTicker(10 * time.Millisecond)
			defer ticker.Stop()
			for range ticker.C {
				if checkRestartCondition() {
					atomic.StoreInt64(&restartTriggered, 1)
					return
				}
			}
		}()

		// Simulate ApplyRelease for multiple modules
		mockApplyRelease := func(_ int) {
			atomic.AddInt64(&rec.activeApplyCount, 1)
			defer func() {
				atomic.AddInt64(&rec.activeApplyCount, -1)
				atomic.StoreInt64(&rec.hasAppliedReleases, 1)
			}()

			moduleStarted <- struct{}{}
			<-allowCompletion
			// Simulate module work
			time.Sleep(5 * time.Millisecond)
		}

		// Start concurrent modules
		for i := 0; i < numModules; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				mockApplyRelease(id)
			}(i)
		}

		// Wait for all modules to start
		for i := 0; i < numModules; i++ {
			<-moduleStarted
		}

		// Trigger restart request while modules are still running
		rec.emitRestart("module updates deployed")

		// Verify restart is requested but not triggered while modules are active
		assert.Equal(t, int64(1), atomic.LoadInt64(&rec.needsRestart))
		assert.Equal(t, int64(numModules), atomic.LoadInt64(&rec.activeApplyCount))
		assert.Equal(t, int64(0), atomic.LoadInt64(&restartTriggered))

		// Allow modules to complete
		close(allowCompletion)
		wg.Wait()

		// Give restart checker time to detect the condition
		time.Sleep(50 * time.Millisecond)

		// Verify restart was triggered after all modules completed
		assert.Equal(t, int64(1), atomic.LoadInt64(&restartTriggered))
		assert.Equal(t, int64(0), atomic.LoadInt64(&rec.activeApplyCount))
		assert.Equal(t, int64(1), atomic.LoadInt64(&rec.hasAppliedReleases))
	})

	t.Run("restart is delayed until all modules complete even with staggered completion", func(t *testing.T) {
		rec := &reconciler{
			log:                log.NewNop(),
			activeApplyCount:   0,
			hasAppliedReleases: 0,
			needsRestart:       0,
			delayTimer:         time.NewTimer(time.Second),
		}

		restartChecks := int64(0)
		restartTriggered := int64(0)

		// Mock the restart condition check
		checkRestartCondition := func() bool {
			atomic.AddInt64(&restartChecks, 1)
			if atomic.LoadInt64(&rec.needsRestart) == 1 &&
				atomic.LoadInt64(&rec.activeApplyCount) == 0 &&
				atomic.LoadInt64(&rec.hasAppliedReleases) == 1 {
				atomic.StoreInt64(&restartTriggered, 1)
				return true
			}
			return false
		}

		// Simulate restart checker
		go func() {
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()
			for range ticker.C {
				if checkRestartCondition() {
					return
				}
			}
		}()

		// Start first module
		atomic.AddInt64(&rec.activeApplyCount, 1)

		// Start second module after delay
		go func() {
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt64(&rec.activeApplyCount, 1)
			defer func() {
				atomic.AddInt64(&rec.activeApplyCount, -1)
				atomic.StoreInt64(&rec.hasAppliedReleases, 1)
			}()
			time.Sleep(30 * time.Millisecond) // Module 2 work
		}()

		// Request restart
		rec.emitRestart("staggered module deployment")

		// Complete first module after some time
		time.Sleep(40 * time.Millisecond)
		atomic.AddInt64(&rec.activeApplyCount, -1)
		atomic.StoreInt64(&rec.hasAppliedReleases, 1)

		// Wait for second module to complete and restart to trigger
		time.Sleep(60 * time.Millisecond)

		// Verify restart was triggered only after all modules completed
		assert.Equal(t, int64(1), atomic.LoadInt64(&restartTriggered))
		assert.Equal(t, int64(0), atomic.LoadInt64(&rec.activeApplyCount))
		assert.Greater(t, atomic.LoadInt64(&restartChecks), int64(5), "restart should have been checked multiple times")
	})
}

const repeatCount = 3

func repeatTest(fn func()) {
	for range repeatCount {
		fn()
	}
}
