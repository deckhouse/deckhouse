module github.com/deckhouse/deckhouse/testing

go 1.13

require (
	github.com/benjamintf1/unmarshalledmatchers v0.0.0-20190408201839-bb1c1f34eaea
	github.com/flant/shell-operator v1.0.0-beta.7.0.20200204141603-35d09bc20f2e // branch: master
	github.com/gammazero/deque v0.0.0-20190521012701-46e4ffb7a622
	github.com/imdario/mergo v0.3.8
	github.com/mohae/deepcopy v0.0.0-20170929034955-c48cc78d4826
	github.com/onsi/ginkgo v1.10.3
	github.com/onsi/gomega v1.7.1
	github.com/otiai10/copy v1.0.2
	github.com/segmentio/go-camelcase v0.0.0-20160726192923-7085f1e3c734
	github.com/tidwall/gjson v1.3.4
	github.com/tidwall/sjson v1.0.4
	golang.org/x/sys v0.0.0-20191110163157-d32e6e3b99c4
	gopkg.in/evanphx/json-patch.v4 v4.5.0
	gopkg.in/yaml.v2 v2.2.7
	gopkg.in/yaml.v3 v3.0.0-20191010095647-fc94e3f71652
	k8s.io/apimachinery v0.0.0-20190409092423-760d1845f48b
	sigs.k8s.io/yaml v1.1.1-0.20191128155103-745ef44e09d6 // branch master, with fixes in yaml.v2.2.7
)

//replace github.com/flant/shell-operator => ../../shell-operator
