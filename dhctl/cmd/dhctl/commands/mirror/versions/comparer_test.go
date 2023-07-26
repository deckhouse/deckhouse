// Copyright 2023 Flant JSC
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

package versions

import (
	"context"
	"os"
	"testing"

	"github.com/Masterminds/semver/v3"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/image"
	"github.com/deckhouse/deckhouse/dhctl/cmd/dhctl/commands/mirror/util"
	"github.com/deckhouse/deckhouse/dhctl/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fileDeckhouseRegistry = "fixtures/deckhouse-registry.tar.gz"
)

func TestVersionsComparer_calculateDiff(t *testing.T) {
	fixtureRegistry := image.MustNewRegistry("file:"+fileDeckhouseRegistry, nil)
	err := fixtureRegistry.Init()
	require.NoError(t, err)

	defer os.RemoveAll(util.TrimTarGzExt(fileDeckhouseRegistry))

	type fields struct {
		source         *image.RegistryConfig
		dest           *image.RegistryConfig
		destListOpts   []image.ListOption
		sourceListOpts []image.ListOption
		sourceCopyOpts []image.CopyOption
	}
	type args struct {
		ctx        context.Context
		minVersion string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    []semver.Version
		wantErr error
	}{
		{
			name: "latest version",
			fields: fields{
				source: fixtureRegistry,
				dest:   image.MustNewRegistry("file:"+t.TempDir(), nil),
			},
			args: args{
				ctx:        context.Background(),
				minVersion: "latest",
			},
			want: []semver.Version{
				*semver.MustParse("v1.46.12"),
				*semver.MustParse("v1.47.2"),
				*semver.MustParse("v1.47.5"),
				*semver.MustParse("v1.48.5"),
				*semver.MustParse("v1.49.1"),
			},
		},
		{
			name: "no min version",
			fields: fields{
				source: fixtureRegistry,
				dest:   image.MustNewRegistry("file:"+t.TempDir(), nil),
			},
			args: args{
				ctx: context.Background(),
			},
			want: []semver.Version{
				*semver.MustParse("v1.44.3"),
				*semver.MustParse("v1.45.10"),
				*semver.MustParse("v1.46.12"),
				*semver.MustParse("v1.47.2"),
				*semver.MustParse("v1.47.5"),
				*semver.MustParse("v1.48.5"),
				*semver.MustParse("v1.49.1"),
			},
		},
		{
			name: "specific min version",
			fields: fields{
				source: fixtureRegistry,
				dest:   image.MustNewRegistry("file:"+t.TempDir(), nil),
			},
			args: args{
				ctx:        context.Background(),
				minVersion: "v1.45.5",
			},
			want: []semver.Version{
				*semver.MustParse("v1.45.10"),
				*semver.MustParse("v1.46.12"),
				*semver.MustParse("v1.47.2"),
				*semver.MustParse("v1.47.5"),
				*semver.MustParse("v1.48.5"),
				*semver.MustParse("v1.49.1"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policyContext, err := image.NewPolicyContext()
			require.NoError(t, err)
			defer policyContext.Destroy()

			v := NewVersionsComparer(
				tt.fields.source, tt.fields.dest, tt.fields.destListOpts,
				tt.fields.sourceListOpts, tt.fields.sourceCopyOpts,
				policyContext, log.GetSilentLogger(),
			)

			got, err := v.calculateDiff(tt.args.ctx, tt.args.minVersion)
			require.ErrorIs(t, err, tt.wantErr)
			if err == nil {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestVersionsComparer_ImagesToCopy(t *testing.T) {
	fixtureRegistry := image.MustNewRegistry("file:"+fileDeckhouseRegistry, nil)
	err := fixtureRegistry.Init()
	require.NoError(t, err)

	defer os.RemoveAll(util.TrimTarGzExt(fileDeckhouseRegistry))

	type fields struct {
		source         *image.RegistryConfig
		dest           *image.RegistryConfig
		destListOpts   []image.ListOption
		sourceListOpts []image.ListOption
		sourceCopyOpts []image.CopyOption
	}
	type args struct {
		ctx        context.Context
		minVersion string
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		versions      []string
		modulesImages []string
		wantErr       error
	}{
		{
			name: "Copy from fixtures to folder without destination and min version",
			fields: fields{
				source: fixtureRegistry,
				dest:   image.MustNewRegistry("file:"+t.TempDir(), nil),
			},
			args: args{
				ctx: context.Background(),
			},
			versions: []string{"v1.44.3", "v1.45.10", "v1.46.12", "v1.47.2", "v1.47.5", "v1.48.5", "v1.49.1"},
			modulesImages: []string{
				"v1.44.3-prometheus-alertmanager", "sha256:c60a0f082d1ff54ccfe4be9ff28c4235aac922907cb50cf224a83994d151a9df",
				"v1.44.3-prometheus-alertsReceiver", "sha256:004996a7e2fbc1996d5b65831ac75d2ae4c0be2ed634a4ee98a1fe71657d6b4d",
				"v1.44.3-prometheus-grafana", "sha256:2c04bfb2fc3797a4e89dcbc847eae4825e8375a25797c53aeff4bceb658295b9",
				"v1.44.3-prometheus-grafanaDashboardProvisioner", "sha256:ef42199e5980e0c2405d606b4b033f45d063d360f6427cc020de15ec0ed1e50c",
				"v1.44.3-prometheus-prometheus", "sha256:8dc29482044ddce32c90f447b92c682dcd03634dd229bd5c185bcbfe7f291fae",
				"v1.44.3-prometheus-trickster", "sha256:2ffc697b32fb6592543fead8c6271fe2d751a424d54d28124a16a78b722c8c4e",
				"v1.45.10-registrypackages-conntrackDebian1462", "sha256:285cc470b351c5661ce4679d6c9f11782f3de0305cf55c98169db5cd83fdd3b9",
				"v1.45.10-registrypackages-containerd1620", "sha256:db912c2134d103fafb7aa9f9203e198f75927d2e443ed0fd1922a8cc802e4252",
				"v1.45.10-registrypackages-containerdAlteros14631El7X8664", "sha256:efdd59c15b0b5f96bbf046a705a23d8e93c6d1f2fcc4d2cc83781c4ee9bfe259",
				"v1.45.10-registrypackages-containerdAlteros161831El7X8664", "sha256:fb63b508d848a96ddbeb6b2d74b7854875a56610534854b6067c099ce9c565f5",
				"v1.45.10-registrypackages-containerdAstra1461Buster", "sha256:77c0512510851d4faefd656342cc6de7b99a8cc85d87bca8e1ba4e7166d99f85",
				"v1.45.10-registrypackages-containerdAstra16181Buster", "sha256:fa9a092d86d5f82039b154c9d0c5f190f29c022a068f435ade2648ae62d4b300",
				"v1.45.10-registrypackages-containerdCentos14631El7X8664", "sha256:56d1d0cf8b8612ddc7269edd1d8e5e0e40c45d372fb36ec47c158aab1d1bfd7b",
				"v1.45.10-registrypackages-containerdCentos14631El8X8664", "sha256:a52e0947af2f74c58ded1994b6de5a1c80c9ebce7e777a2710ddde73db9d9573",
				"v1.45.10-registrypackages-containerdCentos16731El9X8664", "sha256:9c5682cded6b5445fc534dd2fffd052eb02c1495ed2342e6fe6455ddab592823",
				"v1.45.10-registrypackages-containerdDebian1431Stretch", "sha256:e72afbbce2aa6b5b006735e809acca8fca33ec13a451eb263962294b7f855a10",
				"v1.45.10-registrypackages-containerdDebian1461Bullseye", "sha256:78846f66fbb09ac1f4def86285c2301a38e021a0a1ccc9c8b4b97d11a63c8dec",
				"v1.45.10-registrypackages-containerdDebian1461Buster", "sha256:e5ac8ac551ca72b60556b8eb955ac4f459070f699f155304229466978728955a",
				"v1.45.10-registrypackages-containerdRedos14631El7X8664", "sha256:5bc2785185b8c3ab971acbb637b60d3edbd87a2661e4760f7edb413709694fc9",
				"v1.45.10-registrypackages-containerdRedos161831El7X8664", "sha256:213fc8c0757a37cf007db24d6439500729a5187769f5a16f70f6eca80a80bf80",
				"v1.45.10-registrypackages-containerdUbuntu1461Bionic", "sha256:d0f1c2998693252456d897d1b7fefd863f1ca491fc8bfdc42a867265d9acc0f6",
				"v1.45.10-registrypackages-containerdUbuntu1461Focal", "sha256:fe35c6436321b0b277bdd57d31b2a266edf63357100c07dd77b64dbb0e7b3d81",
				"v1.45.10-registrypackages-containerdUbuntu15111Jammy", "sha256:d1038a9f66d2c7f80f51f7e81b650a55c1836c75e733058ccab68f19666a5f4c",
				"v1.45.10-registrypackages-crictl122", "sha256:cb8f6a1813091912cb66a47297355c49774b484bdad92d910f925f44645cc216",
				"v1.45.10-registrypackages-crictl123", "sha256:5b4dbda05e26f5b044b0fd7bb91b6aaf31a1bd3dddec8c0b3aba81e33c6e1cb5",
				"v1.45.10-registrypackages-crictl124", "sha256:3b2f5bc28f4a26348e72224a6d8c75d524b540cad1dd0ce81900091d6c4c2dff",
				"v1.45.10-registrypackages-crictl125", "sha256:4e409447b8dbae92ca60df33c36470d0a832be51afd90bea0566490a13a3f384",
				"v1.45.10-registrypackages-crictl126", "sha256:a3b2de5a86c96cc59715ef8dfe05d7e42f0ff7e43ec3f6a267314b5b02cfc86b",
				"v1.45.10-registrypackages-d8Curl801", "sha256:10473d145bae70a1995d8c2d1c94bd7dbd03840a9b195a3d9b39d8a2eb1347e1",
				"v1.45.10-registrypackages-dockerAlteros1903153El7X866473", "sha256:386db52b3798c27e5361762fe7decde420139e25d16e501be292c3e6f44daa94",
				"v1.45.10-registrypackages-dockerAltlinux2301Alt1X8664", "sha256:f1dbd201ae81eb8f245f3b194935ece51c13b195e2281657adcd2d840adb1f4e",
				"v1.45.10-registrypackages-dockerAstra520101230DebianBuster", "sha256:1102177db0818ebd7978d08d79554aeb2965f5114adf1451d8466ec2fc1fe5df",
				"v1.45.10-registrypackages-dockerCentos1903153El7X86647", "sha256:26a35e37bdddf711fd3f096ba32893e42c0987e5d5d8f636fdfb74d73aa44707",
				"v1.45.10-registrypackages-dockerCentos1903153El8X86648", "sha256:ea77db90763797a6f7296958b52810df12d9b78412aa41c6fca6cb226a864aa0",
				"v1.45.10-registrypackages-dockerCentos2010173El9X86649", "sha256:0b0af4500d131178a606763310109b9b6f56fe7340fbec6e63f788ae8147d3fa",
				"v1.45.10-registrypackages-dockerDebian519031530DebianStretch", "sha256:f7778e27c9d70b6e8beaeac109a54b92ae24b98ef02f90a65521472ad57ed914",
				"v1.45.10-registrypackages-dockerDebian520101230DebianBullseye", "sha256:e6caa2d344cc2eb7a1325708b44237c2751ca504a6578e8b1f03009bd622828d",
				"v1.45.10-registrypackages-dockerDebian520101230DebianBuster", "sha256:f5a51e7dc8f4449be27ccff2b445971ff13925d71f4cc17bde428ddd3b982ad7",
				"v1.45.10-registrypackages-dockerRedos1903153El7X866473", "sha256:a92c5cf3677f61071aee19e1a6233608c39192f3f14f269afe1fc72d5d77bd15",
				"v1.45.10-registrypackages-dockerUbuntu519031330UbuntuBionic", "sha256:720a87f1103e4da8dcc5b71206676a9503297b1b2209f219db8b29bb56045cc3",
				"v1.45.10-registrypackages-dockerUbuntu519031330UbuntuFocal", "sha256:df0c56942825af14f92e889c816b36a075d75328c3258b8e58ad7fadfa7bbca3",
				"v1.45.10-registrypackages-dockerUbuntu520101430UbuntuJammy", "sha256:25185aad4b310d53c96206b1507c9b594b2e312dd9979b7dd8276c61c150a988",
				"v1.45.10-registrypackages-inotifyToolsCentos73149", "sha256:0fe54ccc8aa62246ea91eb32fdfb21b41ca95ca583bd61401d6ed82d8d599f13",
				"v1.45.10-registrypackages-inotifyToolsCentos831419", "sha256:17eb0f148086e44bb864a7e722803b2c5afbb82069a84d38b31c4243ab9ede72",
				"v1.45.10-registrypackages-inotifyToolsCentos9322101", "sha256:f5346628ed5e7540974294b37f68024236957f65b0e176bcb7f27a5de8b0e62e",
				"v1.45.10-registrypackages-jq16", "sha256:b2d917a4475e790174bb5ec984ce5b194b4f27e13c3bdee6570e96e28b565af8",
				"v1.45.10-registrypackages-kubeadm12217", "sha256:f8c8839f5187d29aef9ab3ab7c0e09790ec71bcbe39fa7dca776607c9e5a3d56",
				"v1.45.10-registrypackages-kubeadm12317", "sha256:867386913e8f79a2b2cb6a4186e70061a1f08dd57d9f31a5d829ba5df3b998b4",
				"v1.45.10-registrypackages-kubeadm12415", "sha256:1d3a986e4eefd5ed2869e3bca265d1220e24b2c93bb71d00f06e703f17df9d4c",
				"v1.45.10-registrypackages-kubeadm12511", "sha256:f5578b7140f35bfb2341dc7d7a093582a94cdf930a9b14a49bbd2e25ee88542b",
				"v1.45.10-registrypackages-kubeadm1266", "sha256:4bf16936cd461ef093232cefd842190fc052559e73fd0e9c90a49c36a2de7a60",
				"v1.45.10-registrypackages-kubectl12217", "sha256:9f56673819c91baf0b1685d6fa45a11be0d473541ad890b7e8d0bf5e115638ff",
				"v1.45.10-registrypackages-kubectl12317", "sha256:157ed77491a47ffb7b4796036d461086e0fffe46c367590156a615236a5f8def",
				"v1.45.10-registrypackages-kubectl12415", "sha256:8617e819025844e8e0993d3f49ff1655d3de1e7ba22c76697a0d5ee134b9ac0e",
				"v1.45.10-registrypackages-kubectl12511", "sha256:f758725134173727731ec89c7b96a57b9d336bb31014de5d7a9ee68fd616bbe3",
				"v1.45.10-registrypackages-kubectl1266", "sha256:91ca02eedb35864190e5422cb0e9e0378b7d124600694f191845a162ca6396c4",
				"v1.45.10-registrypackages-kubelet12217", "sha256:7744b215318075ff29c54e42dd6c55ec616e54f11fc9199d32c2d5b4944e0778",
				"v1.45.10-registrypackages-kubelet12317", "sha256:5c5bacb2dea2add6a3454e3990ec45efdacef8748443f8a68cd6d2f19b995d27",
				"v1.45.10-registrypackages-kubelet12415", "sha256:56ec33fe3607dc890ac6b313bad9360f0f8e59e9fcf187734cbe108818327192",
				"v1.45.10-registrypackages-kubelet12511", "sha256:7ffeff870fd11c8e0e65342cd77621a59b3e152230cfb1ef2908c22f1fc05824",
				"v1.45.10-registrypackages-kubelet1266", "sha256:ec71c490fb1083c1f486baa327ce6719603e46bef537c841f80223ef7e51b190",
				"v1.45.10-registrypackages-kubernetesCni120", "sha256:1a4e8bcc5903371b9948823dc2e967e7c8e63c6a40f0e8a3535fe75c3c41ac9c",
				"v1.45.10-registrypackages-tomlMerge01", "sha256:3fb1d05e6666f44c8bd4008b964b288572eb9c61f3a33b807564e5055acf8647",
				"v1.45.10-registrypackages-virtWhatDebian1151Deb9u1", "sha256:1f58f948bbc805133cf59a16fd56994df76ee75ad5f2c6824d030e02a59e87f0",
				"v1.46.12-extendedMonitoring-certExporter", "sha256:b3939039d21b85de4b8da68e8bc6dc2d94a8b71867c1d08c80c61ee3bcb95b59",
				"v1.46.12-extendedMonitoring-eventsExporter", "sha256:847afb18ca32d189c4a5c0dcceeb41c0785b107e0c92d03eba740c0a5baa19d1",
				"v1.46.12-extendedMonitoring-extendedMonitoringExporter", "sha256:0e395a3e4a6629248a9d24e789d5370a2f5166258fd001ac29363e881d0a3fbd",
				"v1.46.12-extendedMonitoring-imageAvailabilityExporter", "sha256:3e0941131ce577c62d1210fa52090e309335366116990bc6bcb531a52461e0aa",
				"v1.47.2-flantIntegration-flantPricing", "sha256:db01c0e6f215b14484bbe5ab4e8435f7f712876fcd326ef6de3a4f3e7c5fc5a2",
				"v1.47.2-flantIntegration-grafanaAgent", "sha256:8fe7cc49e97a2e53b117a3678aae03dcc6d97bd4c5e87bc6c14ba651fa1c7ca5",
				"v1.47.2-flantIntegration-madisonProxy", "sha256:7465496acc468defd9af38b9a0dfd3ac91782d36a79e14c3778d6d6c913c1d75",
				"v1.47.5-linstor-drbdDriverLoader", "sha256:215457a457dd85ca831c5e32408a8b77575ec5679db7437894f4331f3f6554fb",
				"v1.47.5-linstor-drbdReactor", "sha256:42a4b41b7d8dc6b9c6b043b2d1bd10f7ead3b54980c197172acf24c69dff33f5",
				"v1.47.5-linstor-linstorAffinityController", "sha256:1f7ddcb42e122592d6befcbc886356355fe2961bdde45b3d352aa0f0af620937",
				"v1.47.5-linstor-linstorCsi", "sha256:c5bf8ebb289c09835248c19510743936d1005b3bca1dbafa93fc377d632a0c33",
				"v1.47.5-linstor-linstorPoolsImporter", "sha256:b35728b53afb845d15bf69ab0444004ffc7167c48f85cdddf5d2ea2d17e9b368",
				"v1.47.5-linstor-linstorSchedulerAdmission", "sha256:85c8b73af82145bf155904f210ccd44f582a49fd9dec999e421f320ad4f1ad17",
				"v1.47.5-linstor-linstorSchedulerExtender", "sha256:0fd3f457245ae43f68bc9ecadbffeb342d0189609bdde5992de208ecf43e0b1a",
				"v1.47.5-linstor-linstorServer", "sha256:6b88d9df38d17c6015f97e44b59789ffbbe097dd7c3ec411c807186968f88285",
				"v1.47.5-linstor-piraeusHaController", "sha256:42d01231431ed9dbd4eb56c7e56cbb9cddb5a90ab26a2adab9a58f2954966551",
				"v1.47.5-linstor-piraeusOperator", "sha256:50c9330c958a078f58da5b4008ac6c60318bbfc821361e63b07ad58fb6cfe970",
				"v1.47.5-linstor-spaas", "sha256:74f06d60ebe5bad2c8bf038ff47061049df9e8df7f35e2f75ffab4e18577f66e",
				"v1.48.5-terraformManager-baseTerraformManager", "sha256:33667c7720f6e50eaaf243e6d9b92df9bbebedd8f91c294d7ca619d966af0524",
				"v1.48.5-terraformManager-terraformManagerAws", "sha256:dbdaabde7aab26117246de82c8439b6ddeb4bffa5ddee89bc4a2f47cb0f61ee7",
				"v1.48.5-terraformManager-terraformManagerAzure", "sha256:84491aebc73f235c8d5d04a9f43420cb44b4304ad8d2a8bc4347debbcacafeb8",
				"v1.48.5-terraformManager-terraformManagerGcp", "sha256:0cca511b7def15eafcc6714724c121fd7bb08721473b9536b8fcef3bf7a17def",
				"v1.48.5-terraformManager-terraformManagerOpenstack", "sha256:68402acee8a49ccea079915df618524d5f77c56a32513f3c2693126c24168193",
				"v1.48.5-terraformManager-terraformManagerVsphere", "sha256:6a80fec83b7bb3e47156642398158cfd9acddfd6a19f61e9604705a29a18e6cb",
				"v1.48.5-terraformManager-terraformManagerYandex", "sha256:eed6aaa48bb37528edabbcf340a4a5fe960cb328d07c74a9e99998eb6b809289",
				"v1.49.1-kubeDns-coredns", "sha256:d6f765e7774f84a30c2242a07022eb30e4baaf13015ed98342f74279445ed6e8",
				"v1.49.1-kubeDns-resolvWatcher", "sha256:f96fe39cb20dd178963c29c6a0d5542ff2abea1557252a605ed51678d45c07f6",
				"v1.49.1-kubeDns-stsPodsHostsAppenderInitContainer", "sha256:1d8f0ec1d279594b97027732409ac10c8d3b690df82fa39090f66197f654cb67",
				"v1.49.1-kubeDns-stsPodsHostsAppenderWebhook", "sha256:694e4f8a659238d77ee1f519b66856a49ed7edf9c8efd483822f179b73f87200",
			},
		},

		{
			name: "Copy from fixtures to folder with very latest min version",
			fields: fields{
				source: fixtureRegistry,
				dest:   image.MustNewRegistry("file:"+t.TempDir(), nil),
			},
			args: args{
				ctx:        context.Background(),
				minVersion: "v1.49",
			},
			versions: []string{"v1.46.12", "v1.47.2", "v1.47.5", "v1.48.5", "v1.49.1"},
			modulesImages: []string{
				"v1.46.12-extendedMonitoring-certExporter", "sha256:b3939039d21b85de4b8da68e8bc6dc2d94a8b71867c1d08c80c61ee3bcb95b59",
				"v1.46.12-extendedMonitoring-eventsExporter", "sha256:847afb18ca32d189c4a5c0dcceeb41c0785b107e0c92d03eba740c0a5baa19d1",
				"v1.46.12-extendedMonitoring-extendedMonitoringExporter", "sha256:0e395a3e4a6629248a9d24e789d5370a2f5166258fd001ac29363e881d0a3fbd",
				"v1.46.12-extendedMonitoring-imageAvailabilityExporter", "sha256:3e0941131ce577c62d1210fa52090e309335366116990bc6bcb531a52461e0aa",
				"v1.47.2-flantIntegration-flantPricing", "sha256:db01c0e6f215b14484bbe5ab4e8435f7f712876fcd326ef6de3a4f3e7c5fc5a2",
				"v1.47.2-flantIntegration-grafanaAgent", "sha256:8fe7cc49e97a2e53b117a3678aae03dcc6d97bd4c5e87bc6c14ba651fa1c7ca5",
				"v1.47.2-flantIntegration-madisonProxy", "sha256:7465496acc468defd9af38b9a0dfd3ac91782d36a79e14c3778d6d6c913c1d75",
				"v1.47.5-linstor-drbdDriverLoader", "sha256:215457a457dd85ca831c5e32408a8b77575ec5679db7437894f4331f3f6554fb",
				"v1.47.5-linstor-drbdReactor", "sha256:42a4b41b7d8dc6b9c6b043b2d1bd10f7ead3b54980c197172acf24c69dff33f5",
				"v1.47.5-linstor-linstorAffinityController", "sha256:1f7ddcb42e122592d6befcbc886356355fe2961bdde45b3d352aa0f0af620937",
				"v1.47.5-linstor-linstorCsi", "sha256:c5bf8ebb289c09835248c19510743936d1005b3bca1dbafa93fc377d632a0c33",
				"v1.47.5-linstor-linstorPoolsImporter", "sha256:b35728b53afb845d15bf69ab0444004ffc7167c48f85cdddf5d2ea2d17e9b368",
				"v1.47.5-linstor-linstorSchedulerAdmission", "sha256:85c8b73af82145bf155904f210ccd44f582a49fd9dec999e421f320ad4f1ad17",
				"v1.47.5-linstor-linstorSchedulerExtender", "sha256:0fd3f457245ae43f68bc9ecadbffeb342d0189609bdde5992de208ecf43e0b1a",
				"v1.47.5-linstor-linstorServer", "sha256:6b88d9df38d17c6015f97e44b59789ffbbe097dd7c3ec411c807186968f88285",
				"v1.47.5-linstor-piraeusHaController", "sha256:42d01231431ed9dbd4eb56c7e56cbb9cddb5a90ab26a2adab9a58f2954966551",
				"v1.47.5-linstor-piraeusOperator", "sha256:50c9330c958a078f58da5b4008ac6c60318bbfc821361e63b07ad58fb6cfe970",
				"v1.47.5-linstor-spaas", "sha256:74f06d60ebe5bad2c8bf038ff47061049df9e8df7f35e2f75ffab4e18577f66e",
				"v1.48.5-terraformManager-baseTerraformManager", "sha256:33667c7720f6e50eaaf243e6d9b92df9bbebedd8f91c294d7ca619d966af0524",
				"v1.48.5-terraformManager-terraformManagerAws", "sha256:dbdaabde7aab26117246de82c8439b6ddeb4bffa5ddee89bc4a2f47cb0f61ee7",
				"v1.48.5-terraformManager-terraformManagerAzure", "sha256:84491aebc73f235c8d5d04a9f43420cb44b4304ad8d2a8bc4347debbcacafeb8",
				"v1.48.5-terraformManager-terraformManagerGcp", "sha256:0cca511b7def15eafcc6714724c121fd7bb08721473b9536b8fcef3bf7a17def",
				"v1.48.5-terraformManager-terraformManagerOpenstack", "sha256:68402acee8a49ccea079915df618524d5f77c56a32513f3c2693126c24168193",
				"v1.48.5-terraformManager-terraformManagerVsphere", "sha256:6a80fec83b7bb3e47156642398158cfd9acddfd6a19f61e9604705a29a18e6cb",
				"v1.48.5-terraformManager-terraformManagerYandex", "sha256:eed6aaa48bb37528edabbcf340a4a5fe960cb328d07c74a9e99998eb6b809289",
				"v1.49.1-kubeDns-coredns", "sha256:d6f765e7774f84a30c2242a07022eb30e4baaf13015ed98342f74279445ed6e8",
				"v1.49.1-kubeDns-resolvWatcher", "sha256:f96fe39cb20dd178963c29c6a0d5542ff2abea1557252a605ed51678d45c07f6",
				"v1.49.1-kubeDns-stsPodsHostsAppenderInitContainer", "sha256:1d8f0ec1d279594b97027732409ac10c8d3b690df82fa39090f66197f654cb67",
				"v1.49.1-kubeDns-stsPodsHostsAppenderWebhook", "sha256:694e4f8a659238d77ee1f519b66856a49ed7edf9c8efd483822f179b73f87200",
			},
		},

		{
			name: "Copy from fixtures to folder with miv version latest",
			fields: fields{
				source: fixtureRegistry,
				dest:   image.MustNewRegistry("file:"+t.TempDir(), nil),
			},
			args: args{
				ctx:        context.Background(),
				minVersion: "latest",
			},
			versions: []string{"v1.46.12", "v1.47.2", "v1.47.5", "v1.48.5", "v1.49.1"},
			modulesImages: []string{
				"v1.46.12-extendedMonitoring-certExporter", "sha256:b3939039d21b85de4b8da68e8bc6dc2d94a8b71867c1d08c80c61ee3bcb95b59",
				"v1.46.12-extendedMonitoring-eventsExporter", "sha256:847afb18ca32d189c4a5c0dcceeb41c0785b107e0c92d03eba740c0a5baa19d1",
				"v1.46.12-extendedMonitoring-extendedMonitoringExporter", "sha256:0e395a3e4a6629248a9d24e789d5370a2f5166258fd001ac29363e881d0a3fbd",
				"v1.46.12-extendedMonitoring-imageAvailabilityExporter", "sha256:3e0941131ce577c62d1210fa52090e309335366116990bc6bcb531a52461e0aa",
				"v1.47.2-flantIntegration-flantPricing", "sha256:db01c0e6f215b14484bbe5ab4e8435f7f712876fcd326ef6de3a4f3e7c5fc5a2",
				"v1.47.2-flantIntegration-grafanaAgent", "sha256:8fe7cc49e97a2e53b117a3678aae03dcc6d97bd4c5e87bc6c14ba651fa1c7ca5",
				"v1.47.2-flantIntegration-madisonProxy", "sha256:7465496acc468defd9af38b9a0dfd3ac91782d36a79e14c3778d6d6c913c1d75",
				"v1.47.5-linstor-drbdDriverLoader", "sha256:215457a457dd85ca831c5e32408a8b77575ec5679db7437894f4331f3f6554fb",
				"v1.47.5-linstor-drbdReactor", "sha256:42a4b41b7d8dc6b9c6b043b2d1bd10f7ead3b54980c197172acf24c69dff33f5",
				"v1.47.5-linstor-linstorAffinityController", "sha256:1f7ddcb42e122592d6befcbc886356355fe2961bdde45b3d352aa0f0af620937",
				"v1.47.5-linstor-linstorCsi", "sha256:c5bf8ebb289c09835248c19510743936d1005b3bca1dbafa93fc377d632a0c33",
				"v1.47.5-linstor-linstorPoolsImporter", "sha256:b35728b53afb845d15bf69ab0444004ffc7167c48f85cdddf5d2ea2d17e9b368",
				"v1.47.5-linstor-linstorSchedulerAdmission", "sha256:85c8b73af82145bf155904f210ccd44f582a49fd9dec999e421f320ad4f1ad17",
				"v1.47.5-linstor-linstorSchedulerExtender", "sha256:0fd3f457245ae43f68bc9ecadbffeb342d0189609bdde5992de208ecf43e0b1a",
				"v1.47.5-linstor-linstorServer", "sha256:6b88d9df38d17c6015f97e44b59789ffbbe097dd7c3ec411c807186968f88285",
				"v1.47.5-linstor-piraeusHaController", "sha256:42d01231431ed9dbd4eb56c7e56cbb9cddb5a90ab26a2adab9a58f2954966551",
				"v1.47.5-linstor-piraeusOperator", "sha256:50c9330c958a078f58da5b4008ac6c60318bbfc821361e63b07ad58fb6cfe970",
				"v1.47.5-linstor-spaas", "sha256:74f06d60ebe5bad2c8bf038ff47061049df9e8df7f35e2f75ffab4e18577f66e",
				"v1.48.5-terraformManager-baseTerraformManager", "sha256:33667c7720f6e50eaaf243e6d9b92df9bbebedd8f91c294d7ca619d966af0524",
				"v1.48.5-terraformManager-terraformManagerAws", "sha256:dbdaabde7aab26117246de82c8439b6ddeb4bffa5ddee89bc4a2f47cb0f61ee7",
				"v1.48.5-terraformManager-terraformManagerAzure", "sha256:84491aebc73f235c8d5d04a9f43420cb44b4304ad8d2a8bc4347debbcacafeb8",
				"v1.48.5-terraformManager-terraformManagerGcp", "sha256:0cca511b7def15eafcc6714724c121fd7bb08721473b9536b8fcef3bf7a17def",
				"v1.48.5-terraformManager-terraformManagerOpenstack", "sha256:68402acee8a49ccea079915df618524d5f77c56a32513f3c2693126c24168193",
				"v1.48.5-terraformManager-terraformManagerVsphere", "sha256:6a80fec83b7bb3e47156642398158cfd9acddfd6a19f61e9604705a29a18e6cb",
				"v1.48.5-terraformManager-terraformManagerYandex", "sha256:eed6aaa48bb37528edabbcf340a4a5fe960cb328d07c74a9e99998eb6b809289",
				"v1.49.1-kubeDns-coredns", "sha256:d6f765e7774f84a30c2242a07022eb30e4baaf13015ed98342f74279445ed6e8",
				"v1.49.1-kubeDns-resolvWatcher", "sha256:f96fe39cb20dd178963c29c6a0d5542ff2abea1557252a605ed51678d45c07f6",
				"v1.49.1-kubeDns-stsPodsHostsAppenderInitContainer", "sha256:1d8f0ec1d279594b97027732409ac10c8d3b690df82fa39090f66197f654cb67",
				"v1.49.1-kubeDns-stsPodsHostsAppenderWebhook", "sha256:694e4f8a659238d77ee1f519b66856a49ed7edf9c8efd483822f179b73f87200",
			},
		},

		{
			name: "Copy from fixtures to folder with old min version, that diff can not be fullfilled",
			fields: fields{
				source: fixtureRegistry,
				dest:   image.MustNewRegistry("file:"+t.TempDir(), nil),
			},
			args: args{
				ctx:        context.Background(),
				minVersion: "v1.41",
			},
			wantErr: ErrNoVersion,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policyContext, err := image.NewPolicyContext()
			require.NoError(t, err)
			defer policyContext.Destroy()

			v := NewVersionsComparer(
				tt.fields.source, tt.fields.dest, tt.fields.destListOpts,
				tt.fields.sourceListOpts, tt.fields.sourceCopyOpts,
				policyContext, log.GetSilentLogger(),
			)

			got, err := v.ImagesToCopy(tt.args.ctx, tt.args.minVersion)
			require.ErrorIs(t, err, tt.wantErr)
			if err == nil {
				assert.Equal(t, testVersions(tt.fields.source, tt.versions, tt.modulesImages), got)
			}
		})
	}
}

func testVersions(registry *image.RegistryConfig, versions []string, modulesImages []string) []*image.ImageConfig {
	images := make([]*image.ImageConfig, 0, 16)
	images = append(images, image.NewImageConfig(registry, "2", "", "security", "trivy-db"))

	for i := 0; i < len(modulesImages)-1; i += 2 {
		images = append(images, image.NewImageConfig(registry, modulesImages[i], modulesImages[i+1]))
	}

	for _, releaseChannel := range []string{"alpha", "beta", "early-access", "stable", "rock-solid"} {
		images = append(images,
			image.NewImageConfig(registry, releaseChannel, ""),
			image.NewImageConfig(registry, releaseChannel, "", "install"),
			image.NewImageConfig(registry, releaseChannel, "", "release-channel"),
		)
	}
	for _, p := range []string{"install", ""} {
		for _, v := range versions {
			images = append(images,
				image.NewImageConfig(registry, v, "", p),
			)
		}
	}
	return images
}
