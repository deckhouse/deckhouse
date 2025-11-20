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
		got := normalizeManifests(singleDocToManifests(gotB))
		expB, err := os.ReadFile(goldenFile)
		require.NoError(suite.T(), err)
		exp := normalizeManifests(singleDocToManifests(expB))
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

	suite.Run("AutoPatch", func() {
		suite.Run("install new module in manual mode with module release approval annotation", func() {
			suite.setupReleaseController(suite.fetchTestFileData("manual-mode-release-approved.yaml"))

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease(suite.testMRName))
				require.NoError(suite.T(), err)
			})
		})

		suite.Run("install new module in manual mode with module release without approval annotation", func() {
			suite.setupReleaseController(suite.fetchTestFileData("manual-mode-release-not-approved.yaml"))

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(ctx, suite.getModuleRelease(suite.testMRName))
				require.NoError(suite.T(), err)
			})
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

	// Module Release Skip Feature (from to)
	suite.Run("Module Release Skip Feature (from to)", func() {
		// 1) Sequential - should skip intermediate versions when constraint allows jump
		suite.Run("Sequential", func() {
			// deployed 1.67.0, pendings: 1.68.0, 1.69.0, 1.70.0 (with constraint from 1.67 to 1.70)
			testData := suite.fetchTestFileData("from-to-sequential.yaml")
			suite.setupReleaseController(testData)

			repeatTest(func() {
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.67.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.68.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.69.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.70.0"))
				require.NoError(suite.T(), err)
			})
		})

		// 2) Jump through n versions
		suite.Run("Jump through n versions", func() {
			testData := suite.fetchTestFileData("from-to-jump.yaml")
			suite.setupReleaseController(testData)

			repeatTest(func() {
				// deployed is 1.67.5, pendings: 1.68.4, 1.69.10, 1.75.2
				// constraints allow jump from 1.67.x -> 1.75.x, so earlier pendings must be skipped
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.67.5"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.68.4"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.69.10"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.75.2"))
				require.NoError(suite.T(), err)
			})
		})

		// 2.1) Jump through n versions, but approved not latest
		suite.Run("Jump through n versions", func() {
			testData := suite.fetchTestFileData("from-to-jump-approved-not-latest.yaml")
			suite.setupReleaseController(testData)

			repeatTest(func() {
				// deployed is 1.67.5, pendings: 1.75.1, 1.75.2 (approved), 1.75.3
				// release will not deployed and stuck on 1.75.3
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.67.5"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.75.1"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.75.2"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.75.3"))
				require.NoError(suite.T(), err)
			})
		})

		// 3) Jump when deployed is above 'from'
		suite.Run("Jump when deployed is above 'from'", func() {
			testData := suite.fetchTestFileData("from-to-deployed-above-from.yaml")
			suite.setupReleaseController(testData)

			// deployed 1.68.0, pendings: 1.69.0, 1.70.0, 1.71.0 (with constraint from 1.67 to 1.71)
			repeatTest(func() {
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.68.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.69.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.70.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.71.0"))
				require.NoError(suite.T(), err)
			})
		})

		// 4) No jump when deployed is below 'from'
		suite.Run("No jump when deployed is below 'from'", func() {
			testData := suite.fetchTestFileData("from-to-below-from.yaml")
			suite.setupReleaseController(testData)

			repeatTest(func() {
				// deployed is 1.66.0 which is below 'from'; no jumping allowed
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.66.25"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.67.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.68.4"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.75.2"))
				require.NoError(suite.T(), err)
			})
		})

		// 5) No backward jump if 'to' is less than deployed
		suite.Run("No backward jump if 'to' is less than deployed", func() {
			testData := suite.fetchTestFileData("from-to-to-less-than-deployed.yaml")
			suite.setupReleaseController(testData)

			// deployed is 1.76.0, constraints endpoint is lower than deployed, ensure we do not deploy any lower pending
			repeatTest(func() {
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.66.23"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.67.5"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.68.3"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.75.2"))
				require.NoError(suite.T(), err)
			})
		})

		// 6) Several update specs (approved version in second constraint)
		suite.Run("Several update specs, must choose jump to constrainted release", func() {
			testData := suite.fetchTestFileData("from-to-several-update-specs-must-choose-constrainted-release.yaml")
			suite.setupReleaseController(testData)

			// deployed is 1.67.5, pendings: 1.68.4, 1.69.10, 1.70.11
			// constraints allow jump from 1.67.x -> 1.70.x & 1.67.x -> 1.75.x, constrainted release must be chosen
			repeatTest(func() {
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.67.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.68.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.69.3"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.70.0"))
				require.NoError(suite.T(), err)
			})
		})

		// 7) Several update specs (different from maximum leap release is approved and matches constraint)
		suite.Run("Different from constrainted release is approved and matches constraint", func() {
			testData := suite.fetchTestFileData("from-to-several-update-specs-different-release.yaml")
			suite.setupReleaseController(testData)

			// deployed is 1.67.5, pendings: 1.68.4, 1.69.10, 1.70.11
			// constraints allow jump from 1.67.x -> 1.70.x no release can be processed
			repeatTest(func() {
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.67.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.68.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.69.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.70.0"))
				require.NoError(suite.T(), err)
			})
		})

		// 8) Update specis not for constrainted release
		suite.Run("Different from constrainted release is approved and not matches to constrainted release", func() {
			testData := suite.fetchTestFileData("from-to-update-spec-not-for-constrainted-release.yaml")
			suite.setupReleaseController(testData)

			// deployed is 1.67.5, pendings: 1.68.4, 1.69.10, 1.70.11
			// constraints allow jump from 1.67.x -> 1.69.x no release can be processed
			repeatTest(func() {
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.67.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.68.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.69.0"))
				require.NoError(suite.T(), err)
				_, err = suite.ctr.handleRelease(context.TODO(), suite.getModuleRelease("demo-1.70.0"))
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

	// Setting restartReason field in real code causes the process to reboot.
	// And at the next startup, Reconcile will be called for existing objects.
	// Therefore, this condition emulates the behavior in real code.
	for result.Requeue || result.RequeueAfter > 0 || suite.ctr.restartReason != "" {
		suite.ctr.restartReason = ""
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

	// Collect module names from ModuleRelease objects to create ModuleConfig objects
	moduleNames := make(map[string]bool)
	for _, manifest := range manifests {
		obj := suite.assembleInitObject(manifest)
		initObjects = append(initObjects, obj)

		// Check if this is a ModuleRelease and collect the module name
		if mr, ok := obj.(*v1alpha1.ModuleRelease); ok {
			moduleNames[mr.GetModuleName()] = true
		}
	}

	// Create ModuleConfig objects for each module
	for moduleName := range moduleNames {
		moduleConfig := &v1alpha1.ModuleConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name: moduleName,
			},
			Spec: v1alpha1.ModuleConfigSpec{
				Version:  1,
				Settings: map[string]interface{}{},
			},
		}
		initObjects = append(initObjects, moduleConfig)
	}

	err := suite.Suite.SetupNoLock(initObjects)
	require.NoError(suite.T(), err)
	logger := log.NewLogger(log.WithLevel(slog.LevelDebug))

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

// normalizeManifests strips non-deterministic fields before comparison
func normalizeManifests(docs []string) []string {
	result := make([]string, 0, len(docs))
	for _, d := range docs {
		result = append(result, stripDeletionTimestamp(d))
	}

	return result
}

func stripDeletionTimestamp(y string) string {
	// Best-effort: if unmarshal fails, return as-is
	var obj map[string]any
	if err := yaml.Unmarshal([]byte(y), &obj); err != nil {
		return y
	}

	if meta, ok := obj["metadata"].(map[string]any); ok {
		delete(meta, "deletionTimestamp")
	}

	b, err := yaml.Marshal(obj)
	if err != nil {
		return y
	}

	return string(b)
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

const repeatCount = 3

func repeatTest(fn func()) {
	for range repeatCount {
		fn()
	}
}
