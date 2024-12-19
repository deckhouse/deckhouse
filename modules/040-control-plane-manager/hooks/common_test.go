/*
Copyright 2021 Flant JSC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package hooks

import (
	"bytes"
	"context"
	"testing"
	"text/template"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/deckhouse/deckhouse/go_lib/dependency"
)

func Test(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "")
}

// test helpers

func testHelperSetETCDMembers(members []*etcdserverpb.Member) {
	mems := make([]*etcdserverpb.Member, len(members))
	for i, member := range members {
		a := *member
		mems[i] = &a
	}
	dependency.TestDC.EtcdClient.MemberListMock.Set(func(_ context.Context) (*clientv3.MemberListResponse, error) {
		return &clientv3.MemberListResponse{
			Members: mems,
		}, nil
	})
}

func testHelperRegisterEtcdMemberUpdate() {
	dependency.TestDC.EtcdClient.MemberUpdateMock.Set(func(ctx context.Context, id uint64, peers []string) (*clientv3.MemberUpdateResponse, error) {
		resp, _ := dependency.TestDC.EtcdClient.MemberList(ctx)
		members := resp.Members
		for i, member := range members {
			if member.ID != id {
				continue
			}
			member.PeerURLs = peers
			members[i] = member
			break
		}

		testHelperSetETCDMembers(members)
		return nil, nil
	})

	dependency.TestDC.EtcdClient.MemberRemoveMock.Set(func(ctx context.Context, id uint64) (*clientv3.MemberRemoveResponse, error) {
		resp, _ := dependency.TestDC.EtcdClient.MemberList(ctx)
		members := resp.Members
		var index int
		for i, member := range members {
			if member.ID != id {
				continue
			}
			index = i
			break
		}
		members = append(members[:index], members[index+1:]...)
		testHelperSetETCDMembers(members)

		return &clientv3.MemberRemoveResponse{Members: members}, nil
	})

	dependency.TestDC.EtcdClient.CloseMock.Return(nil)
}

func etcdPodManifest(data map[string]interface{}) string {
	podTpl := `
apiVersion: v1
kind: Pod
metadata:
  labels:
    component: etcd
    tier: control-plane
  name: {{ .name }}
  namespace: kube-system
spec:
  {{- if .nodeName }}
  nodeName: {{ .nodeName }}
  {{- end }}
  containers:
  - command:
    - etcd
    {{- if .maxDbSize }}
    - --quota-backend-bytes={{ .maxDbSize }}
    {{- end }}
    image: registry.deckhouse.io/deckhouse/ce:etcd-image
    name: etcd
  {{- if not .podIP }}
  hostNetwork: true
  {{- end }}
status:
  hostIP: {{ .hostIP }}
  phase: Running
  {{- if .podIP }}
  podIP: {{ .podIP }}
  {{- else }}
  podIP: {{ .hostIP }}
  {{- end }}
  podIPs:
  {{- if .podIP }}
  - ip: {{ .podIP }}
  {{- else }}
  - ip: {{ .hostIP }}
  {{- end }}
`
	t := template.New("testetcd_pod_template")
	t, err := t.Parse(podTpl)
	if err != nil {
		panic(err)
	}

	var tpl bytes.Buffer

	err = t.Execute(&tpl, data)
	if err != nil {
		panic(err)
	}

	return tpl.String()
}

const testETCDSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: d8-pki
  namespace: kube-system
data:
  etcd-ca.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUJiRENDQVJhZ0F3SUJBZ0lSQVFBQUFBQUFBQUFBQUFBQUFBQUFBQUF3RFFZSktvWklodmNOQVFFTEJRQXcKR1RFWE1CVUdBMVVFQ2hNT1JHVmphMmh2ZFhObElIUmxjM1F3SWhnUE1EQXdNVEF4TURFd01EQXdNREJhR0E4dwpNREF4TURFd01UQXdNREF3TUZvd0dURVhNQlVHQTFVRUNoTU9SR1ZqYTJodmRYTmxJSFJsYzNRd1hEQU5CZ2txCmhraUc5dzBCQVFFRkFBTkxBREJJQWtFQW0wTmNCTlFOaWJocFExSnJkelBJbFd0OXJ0dTNCRlF6aEpMZm93TkkKUDBzb0RudThOajVwT0dPODQxSmRJei9OaExDdE4xY0RUb29ZUFUvSVBpOEZOd0lEQVFBQm96VXdNekFPQmdOVgpIUThCQWY4RUJBTUNCNEF3RXdZRFZSMGxCQXd3Q2dZSUt3WUJCUVVIQXdFd0RBWURWUjBUQVFIL0JBSXdBREFOCkJna3Foa2lHOXcwQkFRc0ZBQU5CQUl2eXBSM2xIemxBRm9VT2xxNkU2WkZ4YnVneWhqbjF3R21yYlZLUGFWSEwKM2xrdTcyUjlMcS9PaXhFd0hXaHFpZFVqbmg1TTdBOEhxZjVQNytrZW1hVT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
  etcd-ca.key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlCT1FJQkFBSkJBSnREWEFUVURZbTRhVU5TYTNjenlKVnJmYTdidHdSVU00U1MzNk1EU0Q5TEtBNTd2RFkrCmFUaGp2T05TWFNNL3pZU3dyVGRYQTA2S0dEMVB5RDR2QlRjQ0F3RUFBUUpBRjJJSG83cUQ1Mi9jZW9VWkpqU28KU3NpTGZ5QWI2Z3o4VFVVSlpUV0RWZlN4b0I1aTNBSllNTkFXd0FRZHJLdENiaTQwSWI5TFhzZm54Zks5dGN5ZQo0UUloQU1YVWI0K3BwWHBDSXRFY3FGblFMdExyU2pWS1B5cGc4ZHh5YzB3Y29FRzVBaUVBeU9xN09ZSHVrcVc4CndhYTlLYU9ucFRxQmIxNCsrOGtsVVVXdnVlWkJ0bThDSUdXZU9iQVI5RzVZaW9uZnJwcHoxWm1DUXh3Y2gxVzkKZG45R1N2Tk53UVFCQWlCNHlSenJNcUNoU3NBU1QxSXpRUzZjMTNKTzZJTEd6YU1BbS90THNCQmJRd0lnZEpNZAo1bUREVFZ1L3FDVHFUTldqUFRucDhpNXJUVytPZUdLL240MDRnM0k9Ci0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCg==

`
