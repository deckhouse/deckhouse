module github.com/deckhouse/deckhouse

go 1.24.6

require (
	github.com/Masterminds/semver/v3 v3.3.1
	github.com/Masterminds/sprig/v3 v3.3.0
	github.com/benjamintf1/unmarshalledmatchers v0.0.0-20190408201839-bb1c1f34eaea
	github.com/clarketm/json v1.15.7
	github.com/cloudflare/cfssl v1.6.5
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc
	github.com/fatih/color v1.16.0 // indirect
	github.com/flant/addon-operator v1.17.1
	github.com/flant/kube-client v1.5.0
	github.com/flant/shell-operator v1.12.1
	github.com/go-openapi/spec v0.22.0
	github.com/gojuno/minimock/v3 v3.4.7
	github.com/google/go-cmp v0.7.0
	github.com/google/go-containerregistry v0.20.6
	github.com/google/uuid v1.6.0
	github.com/hashicorp/go-multierror v1.1.1
	github.com/iancoleman/strcase v0.3.0
	github.com/imdario/mergo v0.3.16
	github.com/mitchellh/mapstructure v1.5.0 // indirect
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.37.0
	github.com/otiai10/copy v1.14.0
	github.com/pkg/errors v0.9.1
	github.com/sirupsen/logrus v1.9.3 // indirect
	github.com/spaolacci/murmur3 v1.1.0
	github.com/square/go-jose/v3 v3.0.0-20200630053402-0a67ce9b0693
	github.com/stretchr/testify v1.11.1
	github.com/tidwall/gjson v1.18.0
	github.com/tidwall/sjson v1.2.5
	github.com/vmware/govmomi v0.51.0 // indirect
	go.etcd.io/etcd/api/v3 v3.5.16
	go.etcd.io/etcd/client/v3 v3.5.16
	google.golang.org/grpc v1.71.0
	google.golang.org/protobuf v1.36.6 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/yaml.v2 v2.4.0
	gopkg.in/yaml.v3 v3.0.1
	helm.sh/helm/v3 v3.17.4
	k8s.io/api v0.32.10
	k8s.io/apiextensions-apiserver v0.32.10
	k8s.io/apimachinery v0.32.10
	k8s.io/apiserver v0.32.10
	k8s.io/client-go v0.32.10
	k8s.io/utils v0.0.0-20241104100929-3ea5e8cea738
	sigs.k8s.io/yaml v1.4.0
)

require (
	github.com/deckhouse/deckhouse/dhctl v0.0.0-00010101000000-000000000000
	github.com/deckhouse/deckhouse/egress-gateway-agent v0.0.0-00010101000000-000000000000
	github.com/deckhouse/deckhouse/go_lib/cloud-data v0.0.0
	github.com/deckhouse/deckhouse/go_lib/dependency/k8s/drain v0.0.0-00010101000000-000000000000
	github.com/deckhouse/deckhouse/go_lib/dependency/vsphere v0.0.0-00010101000000-000000000000
	github.com/deckhouse/deckhouse/go_lib/registry v0.0.0-00010101000000-000000000000
	github.com/deckhouse/deckhouse/pkg/log v0.1.0
	github.com/deckhouse/deckhouse/pkg/metrics-storage v0.3.0
	github.com/deckhouse/deckhouse/pkg/registry v0.0.0-20251124125852-cdf49be27218
	github.com/deckhouse/module-sdk v0.5.0
	github.com/ettle/strcase v0.2.0
	github.com/evanphx/json-patch v5.9.0+incompatible
	github.com/fsnotify/fsnotify v1.7.0
	github.com/gammazero/deque v1.1.0
	github.com/go-logr/logr v1.4.3
	github.com/go-openapi/swag v0.23.0
	github.com/go-ozzo/ozzo-validation/v4 v4.3.0
	github.com/gofrs/uuid/v5 v5.3.2
	github.com/golang/protobuf v1.5.4
	github.com/itchyny/gojq v0.12.17
	github.com/jonboulle/clockwork v0.4.0
	github.com/shirou/gopsutil/v3 v3.24.5
	github.com/slok/kubewebhook/v2 v2.5.0
	github.com/sylabs/oci-tools v0.16.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.61.0
	go.opentelemetry.io/otel v1.36.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.27.0
	go.opentelemetry.io/otel/sdk v1.36.0
	go.opentelemetry.io/otel/trace v1.36.0
	golang.org/x/exp v0.0.0-20250711185948-6ae5c78190dc
	golang.org/x/mod v0.29.0
	golang.org/x/text v0.31.0
	k8s.io/code-generator v0.32.10
	k8s.io/kube-openapi v0.0.0-20241105132330-32ad38e42d3f
	k8s.io/kubectl v0.32.10
	sigs.k8s.io/controller-runtime v0.20.4
	sigs.k8s.io/json v0.0.0-20241010143419-9aa6b5e7a4b3
	sigs.k8s.io/kustomize/kyaml v0.18.1
)

require k8s.io/klog v1.0.0 // indirect

require (
	cel.dev/expr v0.19.1 // indirect
	dario.cat/mergo v1.0.1 // indirect
	github.com/antlr4-go/antlr/v4 v4.13.0 // indirect
	github.com/deckhouse/lib-gossh v0.0.0-20251020135121-5001bcb43ae8 // indirect
	github.com/fxamacker/cbor/v2 v2.7.0 // indirect
	github.com/go-openapi/swag/conv v0.25.1 // indirect
	github.com/go-openapi/swag/fileutils v0.25.1 // indirect
	github.com/go-openapi/swag/jsonname v0.25.1 // indirect
	github.com/go-openapi/swag/jsonutils v0.25.1 // indirect
	github.com/go-openapi/swag/loading v0.25.1 // indirect
	github.com/go-openapi/swag/mangling v0.25.1 // indirect
	github.com/go-openapi/swag/stringutils v0.25.1 // indirect
	github.com/go-openapi/swag/typeutils v0.25.1 // indirect
	github.com/go-openapi/swag/yamlutils v0.25.1 // indirect
	github.com/go-viper/mapstructure/v2 v2.4.0 // indirect
	github.com/hashicorp/hcl v1.0.1-vault-7 // indirect
	github.com/hashicorp/vault/api v1.20.0 // indirect
	github.com/name212/govalue v1.0.2 // indirect
	github.com/oklog/ulid v1.3.1 // indirect
	github.com/sigstore/sigstore v1.8.8 // indirect
	github.com/sigstore/sigstore/pkg/signature/kms/hashivault v1.8.8 // indirect
	github.com/titanous/rocacheck v0.0.0-20171023193734-afe73141d399 // indirect
	github.com/x448/float16 v0.8.4 // indirect
	go.yaml.in/yaml/v3 v3.0.4 // indirect
	gopkg.in/evanphx/json-patch.v4 v4.12.0 // indirect
	k8s.io/cli-runtime v0.32.10 // indirect
	k8s.io/klog/v2 v2.130.1 // indirect
)

require (
	github.com/AdaLogics/go-fuzz-headers v0.0.0-20230811130428-ced1acdcaa24 // indirect
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/BurntSushi/toml v1.4.0 // indirect
	github.com/DataDog/gostackparse v0.7.0 // indirect
	github.com/GehirnInc/crypt v0.0.0-20230320061759-8cc1b52080c5 // indirect
	github.com/MakeNowJust/heredoc v1.0.0 // indirect
	github.com/Masterminds/goutils v1.1.1 // indirect
	github.com/Masterminds/squirrel v1.5.4 // indirect
	github.com/Masterminds/vcs v1.13.3 // indirect
	github.com/acarl005/stripansi v0.0.0-20180116102854-5a71ef0e047d // indirect
	github.com/alecthomas/chroma/v2 v2.15.0 // indirect
	github.com/alecthomas/template v0.0.0-20190718012654-fb15b899a751 // indirect
	github.com/alecthomas/units v0.0.0-20211218093645-b94a6e3cc137 // indirect
	github.com/alessio/shellescape v1.4.1 // indirect
	github.com/araddon/dateparse v0.0.0-20190622164848-0fb0a474d195 // indirect
	github.com/asaskevich/govalidator v0.0.0-20230301143203-a9d515a09cc2 // indirect
	github.com/avelino/slugify v0.0.0-20180501145920-855f152bd774 // indirect
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/aymanbagabas/go-udiff v0.2.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/bramvdbogaerde/go-scp v1.5.0 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/chai2010/gettext-go v1.0.2 // indirect
	github.com/chanced/caps v1.0.2 // indirect
	github.com/containerd/containerd v1.7.29 // indirect
	github.com/containerd/errdefs v1.0.0 // indirect
	github.com/containerd/log v0.1.0 // indirect
	github.com/containerd/platforms v0.2.1 // indirect
	github.com/containerd/stargz-snapshotter/estargz v0.16.3 // indirect
	github.com/coreos/go-semver v0.3.1 // indirect
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/cpuguy83/go-md2man/v2 v2.0.7 // indirect
	github.com/cyphar/filepath-securejoin v0.3.6 // indirect
	github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy v0.0.0-20240626081445-38c0dcfd3af7 // indirect
	github.com/deckhouse/delivery-kit-sdk v0.0.0-20250916111427-09b1cd34f71f // indirect
	github.com/deckhouse/rootca v0.0.0-20250721220328-2b84d72a5db3 // indirect
	github.com/disintegration/imaging v1.6.2 // indirect
	github.com/distribution/reference v0.6.0 // indirect
	github.com/djherbis/buffer v1.2.0 // indirect
	github.com/djherbis/nio/v3 v3.0.1 // indirect
	github.com/dlclark/regexp2 v1.11.4 // indirect
	github.com/docker/cli v28.3.3+incompatible // indirect
	github.com/docker/distribution v2.8.3+incompatible // indirect
	github.com/docker/docker v28.3.3+incompatible // indirect
	github.com/docker/docker-credential-helpers v0.9.3 // indirect
	github.com/docker/go-connections v0.5.0 // indirect
	github.com/docker/go-metrics v0.0.1 // indirect
	github.com/dominikbraun/graph v0.23.0 // indirect
	github.com/emicklei/go-restful/v3 v3.11.2 // indirect
	github.com/evanphx/json-patch/v5 v5.9.11 // indirect
	github.com/exponent-io/jsonpath v0.0.0-20210407135951-1de76d718b3f // indirect
	github.com/fatih/camelcase v1.0.0 // indirect
	github.com/felixge/httpsnoop v1.0.4 // indirect
	github.com/flopp/go-findfont v0.1.0 // indirect
	github.com/fluxcd/flagger v1.36.1 // indirect
	github.com/fogleman/gg v1.3.0 // indirect
	github.com/go-chi/chi/v5 v5.2.2
	github.com/go-errors/errors v1.5.1 // indirect
	github.com/go-gorp/gorp/v3 v3.1.0 // indirect
	github.com/go-jose/go-jose/v4 v4.0.5 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/go-openapi/analysis v0.24.0 // indirect
	github.com/go-openapi/errors v0.22.3 // indirect
	github.com/go-openapi/jsonpointer v0.22.1 // indirect
	github.com/go-openapi/jsonreference v0.21.2 // indirect
	github.com/go-openapi/loads v0.23.1 // indirect
	github.com/go-openapi/strfmt v0.24.0 // indirect
	github.com/go-openapi/validate v0.25.0 // indirect
	github.com/gobwas/glob v0.2.3 // indirect
	github.com/goccy/go-graphviz v0.2.9 // indirect
	github.com/goccy/go-yaml v1.15.23 // indirect
	github.com/gofrs/flock v0.12.1 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0 // indirect
	github.com/google/btree v1.1.3 // indirect
	github.com/google/cel-go v0.22.0 // indirect
	github.com/google/certificate-transparency-go v1.1.7 // indirect
	github.com/google/gnostic-models v0.6.9-0.20230804172637-c7be7c783f49 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/gookit/color v1.5.4 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/gorilla/websocket v1.5.1 // indirect
	github.com/gosuri/uitable v0.0.4 // indirect
	github.com/gregjones/httpcache v0.0.0-20190611155906-901d90724c79 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.0.1 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.20.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-cleanhttp v0.5.2 // indirect
	github.com/hashicorp/go-retryablehttp v0.7.7 // indirect
	github.com/hashicorp/go-rootcerts v1.0.2 // indirect
	github.com/hashicorp/go-secure-stdlib/parseutil v0.1.7 // indirect
	github.com/hashicorp/go-secure-stdlib/strutil v0.1.2 // indirect
	github.com/hashicorp/go-sockaddr v1.0.2 // indirect
	github.com/hashicorp/go-version v1.2.0 // indirect
	github.com/hofstadter-io/cinful v1.0.0 // indirect
	github.com/huandu/xstrings v1.5.0 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/itchyny/timefmt-go v0.1.6 // indirect
	github.com/jedib0t/go-pretty/v6 v6.5.5 // indirect
	github.com/jellydator/ttlcache/v3 v3.4.0 // indirect
	github.com/jmoiron/sqlx v1.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kennygrant/sanitize v1.2.4 // indirect
	github.com/klauspost/compress v1.18.0 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/lann/builder v0.0.0-20180802200727-47ae307949d0 // indirect
	github.com/lann/ps v0.0.0-20150810152359-62de8c46ede0 // indirect
	github.com/letsencrypt/boulder v0.0.0-20240620165639-de9c06129bec // indirect
	github.com/lib/pq v1.10.9 // indirect
	github.com/liggitt/tabwriter v0.0.0-20181228230101-89fcab3d43de // indirect
	github.com/looplab/fsm v1.0.2 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/lufia/plan9stats v0.0.0-20211012122336-39d0f177ccd0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mattn/go-runewidth v0.0.15 // indirect
	github.com/mitchellh/copystructure v1.2.0 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/mitchellh/go-wordwrap v1.0.1 // indirect
	github.com/mitchellh/reflectwalk v1.0.2 // indirect
	github.com/moby/locker v1.0.1 // indirect
	github.com/moby/spdystream v0.5.0 // indirect
	github.com/moby/term v0.5.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/monochromegane/go-gitignore v0.0.0-20200626010858-205db1a8cc00 // indirect
	github.com/muesli/termenv v0.16.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/mwitkow/grpc-proxy v0.0.0-20230212185441-f345521cb9c9 // indirect
	github.com/mxk/go-flowrate v0.0.0-20140419014527-cca7078d478f // indirect
	github.com/nxadm/tail v1.4.11 // indirect
	github.com/ohler55/ojg v1.26.7 // indirect
	github.com/opencontainers/go-digest v1.0.0 // indirect
	github.com/opencontainers/image-spec v1.1.1 // indirect
	github.com/pelletier/go-toml v1.9.5 // indirect
	github.com/peterbourgon/diskv v2.0.1+incompatible // indirect
	github.com/peterhellberg/link v1.1.0 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/power-devops/perfstat v0.0.0-20210106213030-5aafc221ea8c // indirect
	github.com/prometheus/client_golang v1.22.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.62.0 // indirect
	github.com/prometheus/procfs v0.15.1 // indirect
	github.com/rivo/uniseg v0.4.7 // indirect
	github.com/rogpeppe/go-internal v1.14.1 // indirect
	github.com/rubenv/sql-migrate v1.7.1 // indirect
	github.com/russross/blackfriday/v2 v2.1.0 // indirect
	github.com/ryanuber/go-glob v1.0.0 // indirect
	github.com/samber/lo v1.51.0
	github.com/secure-systems-lab/go-securesystemslib v0.9.0 // indirect
	github.com/shoenig/go-m1cpu v0.1.6 // indirect
	github.com/shopspring/decimal v1.4.0 // indirect
	github.com/sourcegraph/conc v0.3.0 // indirect
	github.com/spf13/cast v1.7.0 // indirect
	github.com/spf13/cobra v1.9.1 // indirect
	github.com/spf13/pflag v1.0.6 // indirect
	github.com/stoewer/go-strcase v1.3.0 // indirect
	github.com/tetratelabs/wazero v1.8.1 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/tklauser/go-sysconf v0.3.12 // indirect
	github.com/tklauser/numcpus v0.6.1 // indirect
	github.com/vbatts/tar-split v0.12.1 // indirect
	github.com/vmware/go-vcloud-director/v3 v3.0.0-alpha.31 // indirect
	github.com/wI2L/jsondiff v0.5.0 // indirect
	github.com/weppos/publicsuffix-go v0.30.3-0.20240510084413-5f1d03393b3d // indirect
	github.com/werf/3p-helm v0.0.0-20251113141050-17dd24e6c20b // indirect
	github.com/werf/common-go v0.0.0-20251113140850-a1a98e909e9b // indirect
	github.com/werf/kubedog v0.13.1-0.20251110132552-6ffc5a117ada // indirect
	github.com/werf/lockgate v0.1.1 // indirect
	github.com/werf/logboek v0.6.1 // indirect
	github.com/werf/nelm v1.17.2-0.20251113141246-0e0ea9049efe
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	github.com/xeipuuv/gojsonschema v1.2.0 // indirect
	github.com/xlab/treeprint v1.2.0 // indirect
	github.com/xo/terminfo v0.0.0-20220910002029-abceb7e1c41e // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	github.com/zmap/zcrypto v0.0.0-20231219022726-a1f61fb1661c // indirect
	github.com/zmap/zlint/v3 v3.6.0 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.16 // indirect
	go.mongodb.org/mongo-driver v1.17.4 // indirect
	go.opentelemetry.io/auto/sdk v1.1.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.28.0 // indirect
	go.opentelemetry.io/otel/metric v1.36.0 // indirect
	go.opentelemetry.io/proto/otlp v1.3.1 // indirect
	go.uber.org/multierr v1.11.0 // indirect
	go.uber.org/zap v1.27.0 // indirect
	golang.org/x/crypto v0.45.0 // indirect
	golang.org/x/image v0.21.0 // indirect
	golang.org/x/net v0.47.0 // indirect
	golang.org/x/oauth2 v0.30.0 // indirect
	golang.org/x/sync v0.18.0
	golang.org/x/sys v0.38.0
	golang.org/x/term v0.37.0 // indirect
	golang.org/x/time v0.12.0 // indirect
	golang.org/x/tools v0.38.0 // indirect
	gomodules.xyz/jsonpatch/v2 v2.4.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20250106144421-5f5ef82da422 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20250428153025-10db94c68c34 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/robfig/cron.v2 v2.0.0-20150107220207-be2e0b0deed5
	gopkg.in/satori/go.uuid.v1 v1.2.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	k8s.io/component-base v0.32.10 // indirect
	k8s.io/gengo/v2 v2.0.0-20240911193312-2b36238f13e9 // indirect
	oras.land/oras-go v1.2.6 // indirect
	sigs.k8s.io/kustomize/api v0.18.0 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.4.2 // indirect
)

replace github.com/deckhouse/deckhouse/dhctl => ./dhctl

replace github.com/deckhouse/deckhouse/go_lib/cloud-data => ./go_lib/cloud-data

replace github.com/deckhouse/deckhouse/egress-gateway-agent => ./ee/se-plus/modules/021-cni-cilium/images/egress-gateway-agent/src

// replace with master branch to work with single dash
replace gopkg.in/alecthomas/kingpin.v2 => github.com/alecthomas/kingpin v1.3.8-0.20200323085623-b6657d9477a6

replace go.cypherpunks.ru/gogost/v5 v5.13.0 => github.com/flant/gogost/v5 v5.13.0

replace github.com/deckhouse/deckhouse/go_lib/registry-packages-proxy => ./go_lib/registry-packages-proxy

replace github.com/deckhouse/deckhouse/go_lib/dependency/k8s/drain => ./go_lib/dependency/k8s/drain

replace github.com/deckhouse/deckhouse/go_lib/dependency/vsphere => ./go_lib/dependency/vsphere

replace github.com/deckhouse/deckhouse/go_lib/registry => ./go_lib/registry
