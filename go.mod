module github.com/deckhouse/deckhouse

go 1.13

require (
	github.com/benjamintf1/unmarshalledmatchers v0.0.0-20190408201839-bb1c1f34eaea
	github.com/fatih/color v1.9.0
	github.com/flant/addon-operator v1.0.0-beta.6 // branch: master
	github.com/flant/shell-operator v1.0.0-beta.9.0.20200501142139-f9ab4893a8ce // branch: master
	github.com/gammazero/deque v0.0.0-20190521012701-46e4ffb7a622
	github.com/ghodss/yaml v1.0.0
	github.com/google/go-cmp v0.4.0
	github.com/iancoleman/strcase v0.0.0-20191112232945-16388991a334
	github.com/imdario/mergo v0.3.8
	github.com/kyokomi/emoji v2.1.0+incompatible
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826
	github.com/onsi/ginkgo v1.12.0
	github.com/onsi/gomega v1.9.0
	github.com/otiai10/copy v1.0.2
	github.com/tidwall/gjson v1.3.4
	github.com/tidwall/sjson v1.0.4
	golang.org/x/sys v0.0.0-20200113162924-86b910548bc1
	gopkg.in/evanphx/json-patch.v4 v4.5.0
	gopkg.in/yaml.v2 v2.2.8
	gopkg.in/yaml.v3 v3.0.0-20191120175047-4206685974f2
	k8s.io/api v0.17.0
	k8s.io/apiextensions-apiserver v0.17.0
	k8s.io/apimachinery v0.17.0
	sigs.k8s.io/yaml v1.1.1-0.20191128155103-745ef44e09d6
)

//replace github.com/flant/shell-operator => ../../shell-operator
