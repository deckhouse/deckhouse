// Copyright 2025 Flant JSC
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

package destroy

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	sdk "github.com/deckhouse/module-sdk/pkg/utils"
	"github.com/google/uuid"
	"github.com/name212/govalue"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/deckhouse/deckhouse/dhctl/pkg/apis/deckhouse/v1"
	"github.com/deckhouse/deckhouse/dhctl/pkg/config"
	"github.com/deckhouse/deckhouse/dhctl/pkg/global"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/actions/entity"
	"github.com/deckhouse/deckhouse/dhctl/pkg/kubernetes/client"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/deckhouse"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/destroy/static"
	"github.com/deckhouse/deckhouse/dhctl/pkg/operations/phases"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/session"
	"github.com/deckhouse/deckhouse/dhctl/pkg/system/node/testssh"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/cache"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/fs"
	"github.com/deckhouse/deckhouse/dhctl/pkg/util/retry"
)

var (
	rootTmpDirStatic = path.Join(os.TempDir(), "dhctl-test-static-destroy")
)

func TestStaticDestroy(t *testing.T) {
	defer func() {
		logger := log.GetDefaultLogger()
		if err := os.RemoveAll(rootTmpDirStatic); err != nil {
			logger.LogErrorF("Couldn't remove temp dir '%s': %v\n", rootTmpDirStatic, err)
			return
		}
		logger.LogInfoF("Tmp dir '%s' removed\n", rootTmpDirStatic)
	}()

	defaultHostBastion := testssh.Bastion{
		Host: bastionHost,
		Port: bastionPort,
		User: bastionUser,
	}

	t.Run("single-master", func(t *testing.T) {
		assertStateHasMetaConfigAndResourcesDestroyed := func(t *testing.T, tst *testStaticDestroyTest) {
			tst.assertResourcesSetDestroyedInCache(t, true)
			tst.assertHasMetaConfigInCache(t, true)
		}

		destroyHost := session.Host{Host: "127.0.0.2", Name: "master-1"}

		t.Run("skip resources returns errors because metaconfig not in cache", func(t *testing.T) {
			hosts := []session.Host{destroyHost}
			params := testStaticDestroyTestParams{
				skipResources:   true,
				destroyOverHost: hosts[0],
				hosts:           hosts,
			}

			tst := createTestStaticDestroyTest(t, params)
			defer tst.clean(t)

			testCreateNodes(t, tst.kubeCl, hosts)
			resources := testCreateResourcesForStatic(t, tst.kubeCl)

			err := tst.destroyer.DestroyCluster(context.TODO(), true)
			require.Error(t, err)
			tst.assertStateCacheIsEmpty(t)
			// skip deleting
			assertResourceExists(t, tst.kubeCl, resources)
			tst.assertCleanCommandRan(t, make([]session.Host, 0))
			tst.assertDownloadDiscoveryIP(t, make([]session.Host, 0))
		})

		noBeforeFunc := func(t *testing.T, tst *testStaticDestroyTest) {}
		noAdditionalAssertFunc := func(t *testing.T, tst *testStaticDestroyTest) {}

		singleMasterTests := []struct {
			name string

			sshOut string
			sshErr error

			cleanScriptShouldRun             bool
			stateCacheShouldEmpty            bool
			resourcesShouldDeleted           bool
			destroyClusterShouldReturnsError bool
			kubeProviderShouldCleaned        bool

			skipResources         bool
			skipResourcesCreating bool

			notOverBastion bool

			before func(t *testing.T, tst *testStaticDestroyTest)
			assert func(t *testing.T, tst *testStaticDestroyTest)
		}{
			{
				name: "happy case",

				sshOut: "ok",
				sshErr: nil,

				cleanScriptShouldRun:      true,
				stateCacheShouldEmpty:     true,
				resourcesShouldDeleted:    true,
				kubeProviderShouldCleaned: true,

				destroyClusterShouldReturnsError: false,

				before: noBeforeFunc,
				assert: noAdditionalAssertFunc,
			},

			{
				name: "happy case without bastion",

				sshOut: "ok",
				sshErr: nil,

				cleanScriptShouldRun:      true,
				stateCacheShouldEmpty:     true,
				resourcesShouldDeleted:    true,
				kubeProviderShouldCleaned: true,

				destroyClusterShouldReturnsError: false,

				notOverBastion: true,

				before: noBeforeFunc,
				assert: noAdditionalAssertFunc,
			},

			{
				name: "resources already deleted",

				sshOut: "ok",
				sshErr: nil,

				cleanScriptShouldRun: true,
				// test that we cannot use kube api for deleting resources
				resourcesShouldDeleted:    false,
				stateCacheShouldEmpty:     true,
				kubeProviderShouldCleaned: true,

				destroyClusterShouldReturnsError: false,

				before: func(t *testing.T, tst *testStaticDestroyTest) {
					tst.setResourcesDestroyed(t)
				},
				assert: noAdditionalAssertFunc,
			},

			{
				name: "metaconfig in cache and skip resources but ips not in cache",

				sshOut: "ok",
				sshErr: nil,

				cleanScriptShouldRun: false,
				// test that we cannot use kube api for deleting resources
				resourcesShouldDeleted:    false,
				stateCacheShouldEmpty:     false,
				kubeProviderShouldCleaned: false,

				destroyClusterShouldReturnsError: true,

				skipResources:         true,
				skipResourcesCreating: true,

				before: func(t *testing.T, tst *testStaticDestroyTest) {
					tst.setResourcesDestroyed(t)
					tst.saveMetaConfigToCache(t)
				},
				assert: func(t *testing.T, tst *testStaticDestroyTest) {
					assertStateHasMetaConfigAndResourcesDestroyed(t, tst)
					tst.assertKubeProviderIsErrorProvider(t)
				},
			},

			{
				name: "metaconfig in cache and skip resources and ips in cache",

				sshOut: "ok",
				sshErr: nil,

				cleanScriptShouldRun: true,
				// test that we cannot use kube api for deleting resources
				resourcesShouldDeleted:    false,
				stateCacheShouldEmpty:     true,
				kubeProviderShouldCleaned: false,

				destroyClusterShouldReturnsError: false,

				skipResources:         true,
				skipResourcesCreating: true,

				before: func(t *testing.T, tst *testStaticDestroyTest) {
					masterIPs := make([]string, 0, len(tst.params.hosts))
					for _, host := range tst.params.hosts {
						masterIPs = append(masterIPs, host.Host)
					}
					tst.generateAndSaveNodeUserToCache(t, testNodeUserSaveParams{
						generate:     false,
						createInKube: false,
						processedIPs: nil,
						masterIPs:    masterIPs,
					})
					tst.setResourcesDestroyed(t)
					tst.saveMetaConfigToCache(t)
				},
				assert: func(t *testing.T, tst *testStaticDestroyTest) {
					tst.assertKubeProviderIsErrorProvider(t)
				},
			},

			{
				name: "clean script returns error",

				sshOut: "error!",
				sshErr: errors.New("error"),

				cleanScriptShouldRun:      true,
				resourcesShouldDeleted:    true,
				stateCacheShouldEmpty:     false,
				kubeProviderShouldCleaned: true,

				destroyClusterShouldReturnsError: true,

				before: noBeforeFunc,
				assert: func(t *testing.T, tst *testStaticDestroyTest) {
					assertStateHasMetaConfigAndResourcesDestroyed(t, tst)
				},
			},
		}

		for _, tt := range singleMasterTests {
			t.Run(tt.name, func(t *testing.T) {
				overBastion := !tt.notOverBastion
				runCleanCommandOverBastion := defaultHostBastion
				if !overBastion {
					runCleanCommandOverBastion = testssh.Bastion{}
				}

				hosts := []session.Host{destroyHost}
				params := testStaticDestroyTestParams{
					skipResources:   tt.skipResources,
					destroyOverHost: hosts[0],
					hosts:           hosts,
					overBastion:     overBastion,
				}

				tst := createTestStaticDestroyTest(t, params)
				defer tst.clean(t)

				testCreateNodes(t, tst.kubeCl, hosts)

				var resources []testCreatedResource
				if !tt.skipResourcesCreating {
					resources = testCreateResourcesForStatic(t, tst.kubeCl)
				}

				tt.before(t, tst)

				tst.addCleanCommand(tst.sshProvider, hosts[0], tt.sshOut, tt.sshErr, tst.logger)

				err := tst.destroyer.DestroyCluster(context.TODO(), true)
				assertClusterDestroyError(t, tt.destroyClusterShouldReturnsError, err)

				tst.assertNodeUserDidNotCreate(t)
				tst.assertStateCache(t, tt.stateCacheShouldEmpty)
				tst.assertDownloadDiscoveryIP(t, make([]session.Host, 0))
				assertResources(t, tst.kubeCl, resources, tt.resourcesShouldDeleted)

				assertOverBastion := func(t *testing.T) {}
				cleanScriptRunOnHosts := make([]session.Host, 0)
				if tt.cleanScriptShouldRun {
					cleanScriptRunOnHosts = append(cleanScriptRunOnHosts, hosts[0])
					assertOverBastion = func(t *testing.T) {
						hostIP := hosts[0].Host
						require.Contains(t, tst.cleanCommandsRanOverBastion, hostIP, "contains host in ran over bastion")
						bastions := tst.cleanCommandsRanOverBastion[hostIP]
						require.Len(t, bastions, 1, "should one bastion")
						require.Equal(t, bastions[0], runCleanCommandOverBastion, "should use or not bastion")
					}
				}
				tst.assertCleanCommandRan(t, cleanScriptRunOnHosts)
				assertOverBastion(t)

				switchedHosts := make([]testHostWithBastion, 0)
				tst.assertClientSwitches(t, switchedHosts)
				tst.assertPrivateKeyWritten(t, len(switchedHosts))
				tst.assertKubeProviderCleaned(t, tt.kubeProviderShouldCleaned, false)

				tt.assert(t, tst)
			})
		}
	})

	t.Run("multi-master", func(t *testing.T) {
		type host struct {
			ip   string
			name string

			sshOut string
			sshErr error

			discoveryIPFile    bool
			discoveryIPFileErr error

			cleanScriptShouldRun bool

			switchToConvergerUser bool
			useBastion            testssh.Bastion

			notCreateNodeUser  bool
			notSavedAsMasterIP bool
		}

		extractMasterIPsSavedFromHosts := func(hosts []host) []string {
			res := make([]string, 0, len(hosts))
			for _, h := range hosts {
				if !h.notSavedAsMasterIP {
					res = append(res, h.ip)
				}
			}
			// sort for prevent flaky tests
			sort.Strings(res)
			return res
		}

		saveNodeUserAndUserExistsInCache := func(t *testing.T, tst *testStaticDestroyTest, hosts []host, processedIPs []string) {
			var processed []session.Host
			if len(processedIPs) > 0 {
				assertStringSliceContainsUniqVals(t, processedIPs, "should have uniq processed ips")
				for _, ip := range processedIPs {
					processed = append(processed, session.Host{Host: ip})
				}
			}

			mastersIPsInCache := extractMasterIPsSavedFromHosts(hosts)
			tst.generateAndSaveNodeUserToCache(t, testNodeUserSaveParams{
				masterIPs:    mastersIPsInCache,
				createInKube: false,
				generate:     true,
				processedIPs: processed,
			})
			tst.setNodeUserExistsInCache(t)
		}

		saveNodeUserResourceDestroyedAndUserExistsInCache := func(t *testing.T, tst *testStaticDestroyTest, hosts []host, processedIPs []string) {
			saveNodeUserAndUserExistsInCache(t, tst, hosts, processedIPs)
			tst.setResourcesDestroyed(t)
		}

		noBeforeFunc := func(t *testing.T, tst *testStaticDestroyTest, hosts []host) {}
		saveMetaConfigInCacheBeforeFunc := func(t *testing.T, tst *testStaticDestroyTest, hosts []host) {
			tst.saveMetaConfigToCache(t)
		}

		noAdditionalAssertFunc := func(t *testing.T, tst *testStaticDestroyTest) {}
		assertKubeProviderIsErrorProvider := func(t *testing.T, tst *testStaticDestroyTest) {
			tst.assertKubeProviderIsErrorProvider(t)
		}

		const (
			destroyOverHostIP  = "127.0.0.2"
			secondMasterHostIP = "127.0.0.3"
			thirdMasterHostIP  = "127.0.0.4"
		)

		destroyOverHost := func(h host) host {
			h.ip = destroyOverHostIP
			h.name = "master-1"

			return h
		}

		secondMasterHost := func(h host) host {
			h.ip = secondMasterHostIP
			h.name = "master-2"

			return h
		}

		thirdMasterHost := func(h host) host {
			h.ip = thirdMasterHostIP
			h.name = "master-3"

			return h
		}

		firstHostBastion := testssh.Bastion{
			Host: destroyOverHostIP,
			Port: inputPort,
			User: inputUser,
		}

		multimasterMasterTests := []struct {
			name string

			hosts          []host
			notOverBastion bool

			resourcesShouldDeleted           bool
			stateCacheShouldEmpty            bool
			nodeUserShouldCreated            bool
			nodeUserShouldSavedInCache       bool
			nodeUserExistsSavedInCache       bool
			kubeProviderShouldCleaned        bool
			resourcesDestroyedShouldSet      bool
			metaConfigSavedInCache           bool
			destroyClusterShouldReturnsError bool

			skipResources bool

			before func(t *testing.T, tst *testStaticDestroyTest, hosts []host)
			assert func(t *testing.T, tst *testStaticDestroyTest)
		}{
			{
				name: "happy case 3 masters",
				hosts: []host{
					destroyOverHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						discoveryIPFile:       true,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						useBastion:            defaultHostBastion,
					}),
					secondMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						useBastion:            defaultHostBastion,
					}),
					thirdMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						useBastion:            defaultHostBastion,
					}),
				},

				stateCacheShouldEmpty:     true,
				nodeUserShouldCreated:     true,
				resourcesShouldDeleted:    true,
				kubeProviderShouldCleaned: true,

				destroyClusterShouldReturnsError: false,

				before: noBeforeFunc,
				assert: noAdditionalAssertFunc,
			},

			{
				name: "happy case 3 masters without bastion",
				hosts: []host{
					destroyOverHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						discoveryIPFile:       true,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						// not use bastion
						useBastion: testssh.Bastion{},
					}),
					secondMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						// over destroyer (first) host
						useBastion: firstHostBastion,
					}),
					thirdMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						// over destroyer (first) host
						useBastion: firstHostBastion,
					}),
				},

				stateCacheShouldEmpty:     true,
				nodeUserShouldCreated:     true,
				resourcesShouldDeleted:    true,
				kubeProviderShouldCleaned: true,

				destroyClusterShouldReturnsError: false,
				notOverBastion:                   true,

				before: noBeforeFunc,
				assert: noAdditionalAssertFunc,
			},

			{
				name: "happy case 2 masters",
				hosts: []host{
					destroyOverHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						discoveryIPFile:       true,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						useBastion:            defaultHostBastion,
					}),
					secondMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						useBastion:            defaultHostBastion,
					}),
				},

				stateCacheShouldEmpty:     true,
				nodeUserShouldCreated:     true,
				resourcesShouldDeleted:    true,
				kubeProviderShouldCleaned: true,

				destroyClusterShouldReturnsError: false,

				before: noBeforeFunc,
				assert: noAdditionalAssertFunc,
			},

			{
				name: "happy case 2 masters without bastion",
				hosts: []host{
					destroyOverHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						discoveryIPFile:       true,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						// not use bastion
						useBastion: testssh.Bastion{},
					}),
					secondMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						// over destroyer (first) host
						useBastion: firstHostBastion,
					}),
				},

				stateCacheShouldEmpty:     true,
				nodeUserShouldCreated:     true,
				resourcesShouldDeleted:    true,
				kubeProviderShouldCleaned: true,

				destroyClusterShouldReturnsError: false,
				notOverBastion:                   true,

				before: noBeforeFunc,
				assert: noAdditionalAssertFunc,
			},

			{
				name: "user cannot create on same nodes",
				hosts: []host{
					destroyOverHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						discoveryIPFile:       false,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						useBastion:            defaultHostBastion,
					}),
					secondMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						notCreateNodeUser:     true,
						useBastion:            defaultHostBastion,
					}),
					thirdMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						useBastion:            defaultHostBastion,
					}),
				},

				stateCacheShouldEmpty:      false,
				nodeUserShouldCreated:      true,
				resourcesShouldDeleted:     false,
				nodeUserShouldSavedInCache: true,
				kubeProviderShouldCleaned:  false,
				metaConfigSavedInCache:     true,

				destroyClusterShouldReturnsError: true,

				before: noBeforeFunc,
				assert: func(t *testing.T, tst *testStaticDestroyTest) {
					tst.assertSetHostsAsProcessedInCache(t, make([]string, 0))
				},
			},

			{
				name: "discovery ip returns error",
				hosts: []host{
					destroyOverHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						discoveryIPFile:       true,
						discoveryIPFileErr:    fmt.Errorf("error"),
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						useBastion:            defaultHostBastion,
					}),
					secondMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						useBastion:            defaultHostBastion,
					}),
					thirdMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						useBastion:            defaultHostBastion,
					}),
				},

				stateCacheShouldEmpty:       false,
				nodeUserShouldCreated:       true,
				nodeUserShouldSavedInCache:  true,
				nodeUserExistsSavedInCache:  true,
				resourcesShouldDeleted:      true,
				kubeProviderShouldCleaned:   true,
				resourcesDestroyedShouldSet: true,
				metaConfigSavedInCache:      true,

				destroyClusterShouldReturnsError: true,

				before: noBeforeFunc,
				assert: noAdditionalAssertFunc,
			},

			{
				name: "restart destroy after fix node user creating on same or all nodes but all clean errors",
				hosts: []host{
					destroyOverHost(host{
						sshOut: "error",
						sshErr: errors.New("error"),
						// first master as last, but discovery ip ran
						discoveryIPFile:       true,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						useBastion:            defaultHostBastion,
					}),
					secondMasterHost(host{
						sshOut: "error",
						sshErr: errors.New("error"),
						// second master return error because we save in cache sorted ips
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						useBastion:            defaultHostBastion,
					}),
					thirdMasterHost(host{
						sshOut: "error",
						sshErr: errors.New("error"),
						// third master not run because we save in cache sorted ips
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						useBastion:            defaultHostBastion,
					}),
				},

				stateCacheShouldEmpty:       false,
				nodeUserShouldCreated:       true,
				resourcesShouldDeleted:      true,
				nodeUserShouldSavedInCache:  true,
				nodeUserExistsSavedInCache:  true,
				kubeProviderShouldCleaned:   true,
				resourcesDestroyedShouldSet: true,
				metaConfigSavedInCache:      true,

				destroyClusterShouldReturnsError: true,

				before: func(t *testing.T, tst *testStaticDestroyTest, hosts []host) {
					saveMetaConfigInCacheBeforeFunc(t, tst, hosts)
					mastersIPsInCache := extractMasterIPsSavedFromHosts(hosts)
					tst.generateAndSaveNodeUserToCache(t, testNodeUserSaveParams{
						masterIPs:    mastersIPsInCache,
						createInKube: true,
						generate:     true,
						processedIPs: nil,
					})
				},
				assert: func(t *testing.T, tst *testStaticDestroyTest) {
					tst.assertNodeUserIsNotUpdated(t, true)
					tst.assertSetHostsAsProcessedInCache(t, make([]string, 0))
				},
			},

			{
				name: "restart destroy first additional master destroyed but second returns error",
				hosts: []host{
					destroyOverHost(host{
						sshOut: "error",
						sshErr: errors.New("error"),
						// first master as last, but discovery ip ran
						discoveryIPFile:       true,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						// node user saved in cache
						notCreateNodeUser: true,
						useBastion:        defaultHostBastion,
					}),
					secondMasterHost(host{
						sshOut: "ok",
						sshErr: nil,
						// second master return error because we save in cache sorted ips
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						// node user saved in cache
						notCreateNodeUser: true,
						useBastion:        defaultHostBastion,
					}),
					thirdMasterHost(host{
						sshOut: "error",
						sshErr: errors.New("error"),
						// third master not run because we save in cache sorted ips
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						// node user saved in cache
						notCreateNodeUser: true,
						useBastion:        defaultHostBastion,
					}),
				},

				stateCacheShouldEmpty: false,
				// node user saved in cache
				nodeUserShouldCreated: false,
				// resources should not destroy because they destroyed and saved on cache
				resourcesShouldDeleted:      false,
				nodeUserShouldSavedInCache:  true,
				nodeUserExistsSavedInCache:  true,
				kubeProviderShouldCleaned:   true,
				resourcesDestroyedShouldSet: true,
				metaConfigSavedInCache:      true,

				destroyClusterShouldReturnsError: true,

				before: func(t *testing.T, tst *testStaticDestroyTest, hosts []host) {
					saveMetaConfigInCacheBeforeFunc(t, tst, hosts)
					saveNodeUserResourceDestroyedAndUserExistsInCache(t, tst, hosts, nil)
				},
				assert: func(t *testing.T, tst *testStaticDestroyTest) {
					tst.assertSetHostsAsProcessedInCache(t, []string{secondMasterHostIP})
				},
			},

			{
				name: "restart destroy first and second additional masters destroyed but third returns error",
				hosts: []host{
					destroyOverHost(host{
						sshOut: "error",
						sshErr: errors.New("error"),
						// first master as last, but discovery ip ran
						discoveryIPFile:       true,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						// node user saved in cache
						notCreateNodeUser: true,
						useBastion:        defaultHostBastion,
					}),
					secondMasterHost(host{
						sshOut: "ok",
						sshErr: nil,
						// not run script because it was saved as processed
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						// node user saved in cache
						notCreateNodeUser: true,
						useBastion:        defaultHostBastion,
					}),
					thirdMasterHost(host{
						sshOut: "ok",
						sshErr: nil,
						// third master not run because we save in cache sorted ips
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						// node user saved in cache
						notCreateNodeUser: true,
						useBastion:        defaultHostBastion,
					}),
				},

				stateCacheShouldEmpty: false,
				// node user saved in cache
				nodeUserShouldCreated: false,
				// resources should not destroy because they destroyed and saved on cache
				resourcesShouldDeleted:      false,
				nodeUserShouldSavedInCache:  true,
				nodeUserExistsSavedInCache:  true,
				kubeProviderShouldCleaned:   true,
				resourcesDestroyedShouldSet: true,
				metaConfigSavedInCache:      true,

				destroyClusterShouldReturnsError: true,

				before: func(t *testing.T, tst *testStaticDestroyTest, hosts []host) {
					saveMetaConfigInCacheBeforeFunc(t, tst, hosts)
					saveNodeUserResourceDestroyedAndUserExistsInCache(t, tst, hosts, []string{secondMasterHostIP})
				},
				assert: func(t *testing.T, tst *testStaticDestroyTest) {
					tst.assertSetHostsAsProcessedInCache(t, []string{
						secondMasterHostIP,
						thirdMasterHostIP,
					})
				},
			},

			{
				name: "first and second additional masters destroyed and restart but third returns error",
				hosts: []host{
					destroyOverHost(host{
						sshOut: "error",
						sshErr: errors.New("error"),
						// first master as last, but discovery ip ran
						discoveryIPFile:       true,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						// node user saved in cache
						notCreateNodeUser: true,
						useBastion:        defaultHostBastion,
					}),
					secondMasterHost(host{
						sshOut: "ok",
						sshErr: nil,
						// not run script because it was saved as processed
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						// node user saved in cache
						notCreateNodeUser: true,
						useBastion:        defaultHostBastion,
					}),
					thirdMasterHost(host{
						sshOut: "ok",
						sshErr: nil,
						// not run script because it was saved as processed
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						// node user saved in cache
						notCreateNodeUser: true,
						useBastion:        defaultHostBastion,
					}),
				},

				stateCacheShouldEmpty: false,
				// node user saved in cache
				nodeUserShouldCreated: false,
				// resources should not destroy because they destroyed and saved on cache
				resourcesShouldDeleted:      false,
				nodeUserShouldSavedInCache:  true,
				nodeUserExistsSavedInCache:  true,
				kubeProviderShouldCleaned:   true,
				resourcesDestroyedShouldSet: true,
				metaConfigSavedInCache:      true,

				destroyClusterShouldReturnsError: true,

				before: func(t *testing.T, tst *testStaticDestroyTest, hosts []host) {
					saveMetaConfigInCacheBeforeFunc(t, tst, hosts)
					saveNodeUserResourceDestroyedAndUserExistsInCache(t, tst, hosts, []string{
						secondMasterHostIP,
						thirdMasterHostIP,
					})
				},
				assert: func(t *testing.T, tst *testStaticDestroyTest) {
					tst.assertSetHostsAsProcessedInCache(t, []string{
						secondMasterHostIP,
						thirdMasterHostIP,
					})
				},
			},

			{
				name: "skip resources passed but any not getting from kube",
				hosts: []host{
					destroyOverHost(host{
						sshOut: "ok",
						sshErr: nil,
						// not ran discovery
						discoveryIPFile:       false,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						notCreateNodeUser:     true,
						useBastion:            defaultHostBastion,
					}),
					secondMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						notCreateNodeUser:     true,
						useBastion:            defaultHostBastion,
					}),
					thirdMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						notCreateNodeUser:     true,
						useBastion:            defaultHostBastion,
					}),
				},

				stateCacheShouldEmpty: true,
				// not created
				nodeUserShouldCreated:       false,
				resourcesShouldDeleted:      false,
				nodeUserShouldSavedInCache:  false,
				nodeUserExistsSavedInCache:  false,
				kubeProviderShouldCleaned:   false,
				resourcesDestroyedShouldSet: false,
				metaConfigSavedInCache:      false,

				destroyClusterShouldReturnsError: true,

				skipResources: true,

				before: noBeforeFunc,
				assert: assertKubeProviderIsErrorProvider,
			},

			{
				name: "skip resources passed but node user not created",
				hosts: []host{
					destroyOverHost(host{
						sshOut: "ok",
						sshErr: nil,
						// not ran discovery
						discoveryIPFile:       false,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						notCreateNodeUser:     true,
						useBastion:            defaultHostBastion,
					}),
					secondMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						notCreateNodeUser:     true,
						useBastion:            defaultHostBastion,
					}),
					thirdMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						notCreateNodeUser:     true,
						useBastion:            defaultHostBastion,
					}),
				},

				stateCacheShouldEmpty:       false,
				nodeUserShouldCreated:       false,
				resourcesShouldDeleted:      false,
				nodeUserShouldSavedInCache:  false,
				nodeUserExistsSavedInCache:  false,
				kubeProviderShouldCleaned:   false,
				resourcesDestroyedShouldSet: false,
				metaConfigSavedInCache:      true,

				destroyClusterShouldReturnsError: true,

				skipResources: true,

				before: saveMetaConfigInCacheBeforeFunc,
				assert: assertKubeProviderIsErrorProvider,
			},

			{
				name: "skip resources passed but resources not deleted and not set in cache",
				hosts: []host{
					destroyOverHost(host{
						sshOut: "ok",
						sshErr: nil,
						// not ran discovery
						discoveryIPFile:       false,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						notCreateNodeUser:     true,
						useBastion:            defaultHostBastion,
					}),
					secondMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						notCreateNodeUser:     true,
						useBastion:            defaultHostBastion,
					}),
					thirdMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  false,
						switchToConvergerUser: false,
						notCreateNodeUser:     true,
						useBastion:            defaultHostBastion,
					}),
				},

				stateCacheShouldEmpty: false,
				// saved in cache
				nodeUserShouldCreated:       false,
				resourcesShouldDeleted:      false,
				nodeUserShouldSavedInCache:  true,
				nodeUserExistsSavedInCache:  true,
				kubeProviderShouldCleaned:   false,
				resourcesDestroyedShouldSet: false,
				metaConfigSavedInCache:      true,

				destroyClusterShouldReturnsError: true,

				skipResources: true,

				before: func(t *testing.T, tst *testStaticDestroyTest, hosts []host) {
					saveMetaConfigInCacheBeforeFunc(t, tst, hosts)
					saveNodeUserAndUserExistsInCache(t, tst, hosts, nil)
				},
				assert: assertKubeProviderIsErrorProvider,
			},

			{
				name: "skip resources passed resources deleted. normal destroy masters",
				hosts: []host{
					destroyOverHost(host{
						sshOut: "ok",
						sshErr: nil,
						// not ran discovery
						discoveryIPFile:       true,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						notCreateNodeUser:     true,
						useBastion:            defaultHostBastion,
					}),
					secondMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						notCreateNodeUser:     true,
						useBastion:            defaultHostBastion,
					}),
					thirdMasterHost(host{
						sshOut:                "ok",
						sshErr:                nil,
						cleanScriptShouldRun:  true,
						switchToConvergerUser: true,
						notCreateNodeUser:     true,
						useBastion:            defaultHostBastion,
					}),
				},

				stateCacheShouldEmpty: true,
				// saved in cache
				nodeUserShouldCreated: false,
				// test that we cannot use kube api for destroy resources
				resourcesShouldDeleted:     false,
				nodeUserShouldSavedInCache: false,
				nodeUserExistsSavedInCache: false,
				// here is error provider
				kubeProviderShouldCleaned:   false,
				resourcesDestroyedShouldSet: false,
				metaConfigSavedInCache:      false,

				destroyClusterShouldReturnsError: false,

				skipResources: true,

				before: func(t *testing.T, tst *testStaticDestroyTest, hosts []host) {
					saveMetaConfigInCacheBeforeFunc(t, tst, hosts)
					saveNodeUserResourceDestroyedAndUserExistsInCache(t, tst, hosts, nil)
				},
				assert: assertKubeProviderIsErrorProvider,
			},
		}

		for _, tt := range multimasterMasterTests {
			t.Run(tt.name, func(t *testing.T) {
				overBastion := !tt.notOverBastion

				hosts := make([]session.Host, 0, len(tt.hosts))
				hostsToCreateNodeUser := make([]session.Host, 0, len(tt.hosts))
				hostsToSwitchToConverger := make([]testHostWithBastion, 0, len(tt.hosts))
				mastersIPSInCache := make([]string, 0, len(tt.hosts))
				sessionHosts := make(map[string]session.Host, len(tt.hosts))

				for _, h := range tt.hosts {
					sh := session.Host{
						Host: h.ip,
						Name: h.name,
					}
					hosts = append(hosts, sh)
					sessionHosts[h.ip] = sh

					if !h.notCreateNodeUser {
						hostsToCreateNodeUser = append(hostsToCreateNodeUser, sh)
					}

					if h.switchToConvergerUser {
						hostsToSwitchToConverger = append(hostsToSwitchToConverger, testHostWithBastion{
							host:    sh,
							bastion: h.useBastion,
						})
					}

					if !h.notSavedAsMasterIP {
						mastersIPSInCache = append(mastersIPSInCache, h.ip)
					}
				}

				params := testStaticDestroyTestParams{
					skipResources:   tt.skipResources,
					destroyOverHost: hosts[0],
					hosts:           hosts,
					overBastion:     overBastion,
				}

				tst := createTestStaticDestroyTest(t, params)
				defer tst.clean(t)

				testCreateNodes(t, tst.kubeCl, hosts)

				resources := testCreateResourcesForStatic(t, tst.kubeCl)

				tt.before(t, tst, tt.hosts)

				hostsWithRunCleanScript := make([]session.Host, 0, len(tt.hosts))
				hostsWithRunDownloadDiscoveryIP := make([]session.Host, 0, len(tt.hosts))
				for _, h := range tt.hosts {
					sh, ok := sessionHosts[h.ip]
					require.True(t, ok)
					tst.addCleanCommand(tst.sshProvider, sh, h.sshOut, h.sshErr, tst.logger)
					if h.cleanScriptShouldRun {
						hostsWithRunCleanScript = append(hostsWithRunCleanScript, sh)
					}

					if h.discoveryIPFile {
						tst.addDiscoveryIPFileDownload(tst.sshProvider, sh, h.discoveryIPFileErr)
						hostsWithRunDownloadDiscoveryIP = append(hostsWithRunDownloadDiscoveryIP, sh)
					}
				}

				ctx := context.TODO()

				var waiter *testNodeUserWaiter
				if len(hostsToCreateNodeUser) > 0 {
					waiter = newTestWaiter()
					waiter.goWaitNodeUserAddUserToNodes(ctx, tst.kubeCl, hostsToCreateNodeUser)
				}

				err := tst.destroyer.DestroyCluster(ctx, true)

				assertClusterDestroyError(t, tt.destroyClusterShouldReturnsError, err)

				if waiter != nil {
					waiter.waitAll()
					require.NoError(t, waiter.getErr(), "user should created set on nodes")
				}

				tst.assertNodeUserCreated(t, tt.nodeUserShouldCreated)
				tst.assertStateCache(t, tt.stateCacheShouldEmpty)
				tst.assertDownloadDiscoveryIP(t, hostsWithRunDownloadDiscoveryIP)
				tst.assertCleanCommandRan(t, hostsWithRunCleanScript)
				assertResources(t, tst.kubeCl, resources, tt.resourcesShouldDeleted)
				tst.assertClientSwitches(t, hostsToSwitchToConverger)
				tst.assertPrivateKeyWritten(t, len(hostsToSwitchToConverger))
				tst.assertNodeUserSavedInCache(t, tt.nodeUserShouldSavedInCache)
				if tt.nodeUserShouldSavedInCache {
					tst.assertMasterIPsSavedInCache(t, mastersIPSInCache)
				}
				tst.assertNodeUserExistsSavedInCache(t, tt.nodeUserExistsSavedInCache)
				tst.assertKubeProviderCleaned(t, tt.kubeProviderShouldCleaned, false)
				tst.assertResourcesSetDestroyedInCache(t, tt.resourcesDestroyedShouldSet)
				tst.assertHasMetaConfigInCache(t, tt.metaConfigSavedInCache)

				tt.assert(t, tst)
			})
		}
	})

}

type testStaticDestroyTestParams struct {
	skipResources bool
	overBastion   bool

	destroyOverHost session.Host

	hosts []session.Host
}

type testNodeUserWaiter struct {
	mu  sync.Mutex
	wg  *sync.WaitGroup
	err error
}

func newTestWaiter() *testNodeUserWaiter {
	return &testNodeUserWaiter{
		wg: new(sync.WaitGroup),
	}
}

func (w *testNodeUserWaiter) setErr(err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.err = err
}

func (w *testNodeUserWaiter) getErr() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.err
}

func (w *testNodeUserWaiter) waitAll() {
	w.wg.Wait()
}

func (w *testNodeUserWaiter) goWaitNodeUserAddUserToNodes(ctx context.Context, kubeCl *client.KubernetesClient, hosts []session.Host) {
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()
		err := retry.NewSilentLoop("wait node user", 20, 500*time.Millisecond).RunContext(ctx, func() error {
			_, err := kubeCl.Dynamic().Resource(v1.NodeUserGVR).Get(ctx, global.ConvergeNodeUserName, metav1.GetOptions{})
			return err
		})
		if err != nil {
			w.setErr(err)
			return
		}

		for _, host := range hosts {
			node, err := kubeCl.CoreV1().Nodes().Get(ctx, host.Name, metav1.GetOptions{})
			if err != nil {
				w.setErr(err)
				return
			}

			if len(node.Annotations) == 0 {
				node.Annotations = make(map[string]string)
			}

			node.Annotations[global.ConvergerNodeUserAnnotation] = "true"
			_, err = kubeCl.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
			if err != nil {
				w.setErr(err)
				return
			}
		}

		w.setErr(nil)
	}()
}

type testStaticDestroyTest struct {
	*baseTest

	params testStaticDestroyTestParams

	destroyer *ClusterDestroyer

	kubeCl *client.KubernetesClient

	sshProvider *testssh.SSHProvider

	cleanCommandsRanOnHosts     map[string][]struct{}
	cleanCommandsRanOverBastion map[string][]testssh.Bastion

	downloadDiscoveryIPRanOnHosts     map[string][]struct{}
	downloadDiscoveryIPRanOverBastion map[string][]testssh.Bastion

	nodeUser      *v1.NodeUser
	nodeUserCreds *static.NodesWithCredentials
}

func (ts *testStaticDestroyTest) setNodeUserExistsInCache(t *testing.T) {
	require.False(t, govalue.IsNil(ts.stateCache))

	err := static.NewDestroyState(ts.stateCache).SetNodeUserExists()
	require.NoError(t, err, "node user exists flag should in cache")
}

type testNodeUserSaveParams struct {
	masterIPs    []string
	processedIPs []session.Host
	createInKube bool
	generate     bool
}

func (ts *testStaticDestroyTest) generateAndSaveNodeUserToCache(t *testing.T, params testNodeUserSaveParams) {
	require.False(t, govalue.IsNil(ts.stateCache))

	var nodeUser *v1.NodeUser
	var credentials *v1.NodeUserCredentials

	if params.generate {
		var err error
		nodeUser, credentials, err = v1.GenerateNodeUser(v1.ConvergerNodeUser())
		require.NoError(t, err, "should generated")

		ts.nodeUser = nodeUser
	}

	ips := make([]entity.NodeIP, 0, len(params.masterIPs))
	for _, masterIP := range params.masterIPs {
		ips = append(ips, entity.NodeIP{
			InternalIP: masterIP,
		})
	}

	credsToSave := &static.NodesWithCredentials{
		NodeUser:     credentials,
		IPs:          ips,
		ProcessedIPS: params.processedIPs,
	}

	err := static.NewDestroyState(ts.stateCache).SaveNodeUser(credsToSave)
	require.NoError(t, err, "creds should save in cache")

	ts.nodeUserCreds = credsToSave

	if !params.generate || !params.createInKube {
		return
	}

	require.False(t, govalue.IsNil(ts.kubeCl))
	err = entity.CreateOrUpdateNodeUser(context.TODO(), newFakeKubeClientProvider(ts.kubeCl), ts.nodeUser, retry.NewEmptyParams())
	require.NoError(t, err, "create or update node user")
}

func (ts *testStaticDestroyTest) assertNodeUserIsNotUpdated(t *testing.T, checkInKube bool) {
	require.False(t, govalue.IsNil(ts.stateCache))
	require.False(t, govalue.IsNil(ts.nodeUserCreds))

	nodeUserCreds, err := static.NewDestroyState(ts.stateCache).NodeUser()
	require.NoError(t, err, "node user should save in cache")
	require.Equal(t, *ts.nodeUserCreds, *nodeUserCreds, "node user should not change")

	if !checkInKube {
		return
	}

	require.False(t, govalue.IsNil(ts.kubeCl))
	require.False(t, govalue.IsNil(ts.nodeUser))
	nodeUserUnstruct, err := ts.kubeCl.Dynamic().Resource(v1.NodeUserGVR).Get(context.TODO(), ts.nodeUser.GetName(), metav1.GetOptions{})
	require.NoError(t, err, "node user should in kubernetes cluster")

	nodeUser := v1.NodeUser{}
	err = sdk.FromUnstructured(nodeUserUnstruct, &nodeUser)
	require.NoError(t, err, "node user should unmarshal")

	require.Equal(t, ts.nodeUser.Spec, nodeUser.Spec, "node user specs should not change in cluster")
}

func (ts *testStaticDestroyTest) assertNodeUserExistsSavedInCache(t *testing.T, saved bool) {
	require.False(t, govalue.IsNil(ts.stateCache))

	exists := static.NewDestroyState(ts.stateCache).IsNodeUserExists()
	require.Equal(t, saved, exists, "node user exists flag")
}

func (ts *testStaticDestroyTest) assertNodeUserSavedInCache(t *testing.T, saved bool) *static.NodesWithCredentials {
	require.False(t, govalue.IsNil(ts.stateCache))

	state := static.NewDestroyState(ts.stateCache)
	nodeUser, err := state.NodeUser()
	if !saved {
		require.Error(t, err, "node user should not save in cache")
		return nil
	}

	require.NoError(t, err, "node user should save in cache")
	require.NotNil(t, nodeUser, "node user should save in cache")
	require.NotNil(t, nodeUser.NodeUser, "node user should save in cache")
	require.Equal(t, global.ConvergeNodeUserName, nodeUser.NodeUser.Name, "node user correct name should save in cache")
	require.Equal(t, []string{global.MasterNodeGroupName}, nodeUser.NodeUser.NodeGroups, "node user correct groups should save in cache")
	require.NotEmpty(t, nodeUser.NodeUser.Password, "node user password should save in cache")
	require.NotEmpty(t, nodeUser.NodeUser.PrivateKey, "node user, private key should save in cache")
	require.NotEmpty(t, nodeUser.NodeUser.PublicKey, "node user public key should save in cache")

	return nodeUser
}

func (ts *testStaticDestroyTest) assertSetHostsAsProcessedInCache(t *testing.T, hosts []string) {
	assertStringSliceContainsUniqVals(t, hosts, "should have uniq hosts as processed")

	nodeUser := ts.assertNodeUserSavedInCache(t, true)
	require.NotNil(t, nodeUser)

	require.Len(t, nodeUser.ProcessedIPS, len(hosts))

	processedMap := make(map[string]struct{})
	for _, p := range nodeUser.ProcessedIPS {
		processedMap[p.Host] = struct{}{}
	}

	for _, h := range hosts {
		require.Contains(t, processedMap, h)
	}
}

func (ts *testStaticDestroyTest) assertMasterIPsSavedInCache(t *testing.T, ips []string) {
	nodeUser := ts.assertNodeUserSavedInCache(t, true)
	require.NotNil(t, nodeUser, "node user should save in cache")

	require.Len(t, nodeUser.IPs, len(ips), "node user master ips should save in cache")

	ipsMap := make(map[string]struct{})
	for _, p := range nodeUser.IPs {
		ipsMap[p.InternalIP] = struct{}{}
	}

	for _, ip := range ips {
		require.Contains(t, ipsMap, ip, "node user master ip should save in cache", ip)
	}
}

type testHostWithBastion struct {
	host    session.Host
	bastion testssh.Bastion
}

func (ts *testStaticDestroyTest) assertClientSwitches(t *testing.T, hosts []testHostWithBastion) {
	require.False(t, govalue.IsNil(ts.sshProvider))

	switches := ts.sshProvider.Switches()
	require.Len(t, hosts, len(switches), "should have len switches")

	if len(hosts) == 0 {
		return
	}

	privateKeys := make(map[string]struct{})
	hostsMap := make(map[string]testssh.Bastion)

	for _, h := range hosts {
		hostsMap[h.host.Host] = h.bastion
	}

	for _, s := range switches {
		require.NotNil(t, s.Session, "session")

		hostIP := s.Session.Host()
		require.Contains(t, hostsMap, hostIP, "host should present")
		bastion := hostsMap[hostIP]
		require.False(t, bastion.NoSession, "bastion should not have session")

		require.Equal(t, bastion.Host, s.Session.BastionHost, "bastion host")
		require.Equal(t, bastion.User, s.Session.BastionUser, "bastion user")
		require.Equal(t, bastion.Port, s.Session.BastionPort, "bastion port")
		require.Equal(t, global.ConvergeNodeUserName, s.Session.User, "user name")
		require.Equal(t, inputPort, s.Session.Port, "input port")

		for _, pk := range s.PrivateKeys {
			privateKeys[pk.Key] = struct{}{}
		}
	}

	// all user passed keys and converger private key
	// for every host with converge user we create different key
	require.Len(t, privateKeys, len(inputPrivateKeys)+len(hosts), "should have all private keys")
	for _, pk := range inputPrivateKeys {
		require.Contains(t, privateKeys, pk, "should have private key", pk)
	}
}

func (ts *testStaticDestroyTest) assertPrivateKeyWritten(t *testing.T, keysCount int) {
	require.NotEmpty(t, ts.tmpDir)

	destroyTmpDir := filepath.Join(ts.tmpDir, "destroy")
	tmpDirStat, err := os.Stat(destroyTmpDir)

	if keysCount == 0 {
		require.Error(t, err, "destroy dir should not create")
		require.True(t, errors.Is(err, os.ErrNotExist), "destroy dir should not create")
		return
	}

	require.NoError(t, err, "destroy dir should created")
	require.True(t, tmpDirStat.IsDir(), "destroy dir should created")

	keysPaths := make([]string, 0)
	err = filepath.Walk(destroyTmpDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		if strings.HasPrefix(info.Name(), "id_rsa_destroyer.key.") {
			keysPaths = append(keysPaths, path)
		}

		return nil
	})

	require.NoError(t, err, "private keys should be written")
	require.Len(t, keysPaths, keysCount, "private keys should be written")
}

func (ts *testStaticDestroyTest) assertNodeUserCreated(t *testing.T, created bool) {
	require.False(t, govalue.IsNil(ts.kubeCl))

	_, err := ts.kubeCl.Dynamic().Resource(v1.NodeUserGVR).Get(context.TODO(), global.ConvergeNodeUserName, metav1.GetOptions{})

	if created {
		require.NoError(t, err, "node user should created ")
		return
	}

	require.Error(t, err, "node user should not created ")
	require.True(t, k8errors.IsNotFound(err), "node user should not created")
}

func (ts *testStaticDestroyTest) assertNodeUserDidNotCreate(t *testing.T) {
	ts.assertNodeUserCreated(t, false)
}

func testAddRunToMap[T any](hostsMap map[string][]T, hostIP string, val T) {
	list, ok := hostsMap[hostIP]
	if !ok || len(list) == 0 {
		list = make([]T, 0, 1)
	}
	list = append(list, val)
	hostsMap[hostIP] = list
}

func assertHostsMapRunOnce[T any](t *testing.T, expectedHosts []session.Host, hostsMap map[string][]T, msg string) {
	require.Len(t, expectedHosts, len(hostsMap), msg)

	if len(expectedHosts) == 0 {
		return
	}

	for _, h := range expectedHosts {
		require.Contains(t, hostsMap, h.Host, msg, h.Host)
		require.Len(t, hostsMap[h.Host], 1, msg)
	}
}

func (ts *testStaticDestroyTest) assertCleanCommandRan(t *testing.T, hosts []session.Host) {
	assertHostsMapRunOnce(t, hosts, ts.cleanCommandsRanOnHosts, "clean command")
}

func (ts *testStaticDestroyTest) assertDownloadDiscoveryIP(t *testing.T, hosts []session.Host) {
	assertHostsMapRunOnce(t, hosts, ts.downloadDiscoveryIPRanOnHosts, "download discovery api")
}

func (ts *testStaticDestroyTest) addDiscoveryIPFileDownload(sshProvider *testssh.SSHProvider, forHost session.Host, returnErr error) {
	sshProvider.SetFileProvider(forHost.Host, func(bastion testssh.Bastion) *testssh.File {
		download := func(srcPath string) ([]byte, error) {
			if srcPath != "/var/lib/bashible/discovered-node-ip" {
				return nil, fmt.Errorf("'%s' file not found", srcPath)
			}

			hostIP := forHost.Host

			testAddRunToMap(ts.downloadDiscoveryIPRanOnHosts, hostIP, struct{}{})
			testAddRunToMap(ts.downloadDiscoveryIPRanOverBastion, hostIP, bastion)

			if returnErr != nil {
				return nil, returnErr
			}

			return []byte(forHost.Host), nil
		}

		upload := func(data []byte, dstPath string) error {
			return fmt.Errorf("We should not upload any files to server during destroy static: '%s'", dstPath)
		}

		return testssh.NewFile(upload, download)
	})
}

func (ts *testStaticDestroyTest) runCleanCommand(hostIP string, bastion testssh.Bastion, msg string, logger log.Logger) {
	testAddRunToMap(ts.cleanCommandsRanOnHosts, hostIP, struct{}{})
	testAddRunToMap(ts.cleanCommandsRanOverBastion, hostIP, bastion)
	logger.LogInfoLn(msg)
}

func (ts *testStaticDestroyTest) addCleanCommand(sshProvider *testssh.SSHProvider, forHost session.Host, out string, err error, logger log.Logger) {
	sshProvider.AddCommandProvider(forHost.Host, func(bastion testssh.Bastion, scriptPath string, args ...string) *testssh.Command {
		if !testIsCleanCommand(scriptPath) {
			return nil
		}

		hostIP := forHost.Host

		cmd := testssh.NewCommand([]byte(out))
		if err != nil {
			cmd.WithErr(err).WithRun(func() {
				ts.runCleanCommand(hostIP, bastion, "Clean command error", logger)
			})

			return cmd
		}

		return cmd.WithErr(nil).WithRun(func() {
			ts.runCleanCommand(hostIP, bastion, "Clean command success", logger)
		})
	})
}

func createTestStaticDestroyTest(t *testing.T, params testStaticDestroyTestParams) *testStaticDestroyTest {
	logger := log.NewInMemoryLoggerWithParent(log.GetDefaultLogger())

	stateCache := cache.NewTestCache()

	kubeCl := testCreateFakeKubeClient()
	kubeClProvider := newFakeKubeClientProvider(kubeCl)

	ctx := context.TODO()

	clusterUUID := uuid.Must(uuid.NewRandom()).String()

	testCreateClusterConfigSecret(t, kubeCl, staticClusterGeneralConfigYAML)

	testCreateClusterUUIDCM(t, kubeCl, clusterUUID)

	metaConfig, err := config.ParseConfigFromCluster(ctx, kubeCl, config.DummyPreparatorProvider())
	require.NoError(t, err)

	const commanderMode = false

	loaderParams := &stateLoaderParams{
		commanderMode:   commanderMode,
		commanderParams: nil,
		stateCache:      stateCache,
		logger:          logger,
		skipResources:   params.skipResources,
		forceFromCache:  true,
	}

	loader, kubeProviderForInfraDestroyer, err := initStateLoader(ctx, loaderParams, kubeClProvider)
	require.NoError(t, err)

	loggerProvider := log.SimpleLoggerProvider(logger)
	pipeline := phases.NewDummyDefaultPipelineProviderOpts(
		phases.WithPipelineName("static destroy"),
		phases.WithPipelineLoggerProvider(loggerProvider),
	)()

	phaseActionProvider := phases.NewPhaseActionProviderFromPipeline(pipeline)
	d8State := deckhouse.NewState(stateCache)

	d8Destroyer := deckhouse.NewDestroyer(deckhouse.DestroyerParams{
		CommanderMode: commanderMode,
		CommanderUUID: uuid.Nil,
		SkipResources: params.skipResources,

		State: d8State,

		LoggerProvider:       loggerProvider,
		KubeProvider:         kubeClProvider,
		PhasedActionProvider: phaseActionProvider,
	})

	sshProvider := testCreateDefaultTestSSHProvider(params.destroyOverHost, params.overBastion)

	i := rand.New(rand.NewSource(time.Now().UnixNano()))
	tmpDir, err := fs.RandomTmpDirWithNRunes(rootTmpDirStatic, fmt.Sprintf("%d", i), 15)
	require.NoError(t, err)

	logger.LogInfoF("Tmp dir: '%s'\n", tmpDir)

	infraProvider := &infraDestroyerProvider{
		stateCache:           stateCache,
		loggerProvider:       loggerProvider,
		kubeProvider:         kubeProviderForInfraDestroyer,
		phasesActionProvider: phaseActionProvider,
		commanderMode:        commanderMode,
		skipResources:        params.skipResources,
		cloudStateProvider:   nil,
		sshClientProvider:    sshProvider,
		tmpDir:               tmpDir,
		staticLoopsParams: static.LoopsParams{
			NodeUser: retry.NewEmptyParams(
				retry.WithWait(2*time.Second),
				retry.WithAttempts(4),
			),
			DestroyMaster: retry.NewEmptyParams(
				retry.WithWait(1*time.Second),
				retry.WithAttempts(1),
			),
			GetMastersIPs: retry.NewEmptyParams(
				retry.WithWait(1*time.Second),
				retry.WithAttempts(2),
			),
		},
	}

	destroyer := &ClusterDestroyer{
		stateCache:       stateCache,
		configPreparator: loader,

		pipeline: pipeline,

		d8Destroyer:   d8Destroyer,
		infraProvider: infraProvider,
	}

	tst := &testStaticDestroyTest{
		baseTest: &baseTest{
			logger:       logger,
			stateCache:   stateCache,
			tmpDir:       tmpDir,
			kubeProvider: kubeProviderForInfraDestroyer,
			metaConfig:   metaConfig,
		},

		params: params,

		destroyer: destroyer,

		kubeCl: kubeCl,

		sshProvider: sshProvider,

		cleanCommandsRanOnHosts:     make(map[string][]struct{}),
		cleanCommandsRanOverBastion: make(map[string][]testssh.Bastion),

		downloadDiscoveryIPRanOnHosts:     make(map[string][]struct{}),
		downloadDiscoveryIPRanOverBastion: make(map[string][]testssh.Bastion),
	}

	return tst
}

func testCreateResourcesForStatic(t *testing.T, kubeCl *client.KubernetesClient) []testCreatedResource {
	return append(testCreateResourcesGeneral(t, kubeCl), testCreateCAPIResources(t, kubeCl)...)
}

func testCreateNodes(t *testing.T, kubeCl *client.KubernetesClient, hosts []session.Host) {
	names := make(map[string]struct{})
	ips := make(map[string]struct{})
	for _, host := range hosts {
		names[host.Name] = struct{}{}
		ips[host.Host] = struct{}{}
	}

	require.Len(t, names, len(hosts), hosts)
	require.Len(t, ips, len(hosts), hosts)

	for _, host := range hosts {
		obj := corev1.Node{
			ObjectMeta: metav1.ObjectMeta{
				Name: host.Name,
				Labels: map[string]string{
					"node.deckhouse.io/group": "master",
				},
			},
			Status: corev1.NodeStatus{
				Addresses: []corev1.NodeAddress{
					{Address: host.Host, Type: corev1.NodeInternalIP},
					{Address: host.Name, Type: corev1.NodeHostName},
				},
			},
		}

		_, err := kubeCl.CoreV1().Nodes().Create(context.TODO(), &obj, metav1.CreateOptions{})
		require.NoError(t, err, host.Name)
	}

	nodes, err := kubeCl.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{})
	require.NoError(t, err, hosts)
	require.Len(t, nodes.Items, len(hosts))
	for _, node := range nodes.Items {
		require.Len(t, node.Status.Addresses, 2)
	}
}
