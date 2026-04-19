/*
Copyright 2025 Flant JSC

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
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

var _ = Describe("openvpn :: hooks :: check_server_cert_expiry ::", func() {

	const (
		d8OpenvpnNamespace = `
---
apiVersion: v1
kind: Namespace
metadata:
  name: d8-openvpn
  labels:
    heritage: deckhouse
    module: openvpn
spec:
  finalizers:
  - kubernetes
`
		emptySecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: openvpn-pki-server
  namespace: d8-openvpn
type: Opaque
data: {}
`
		invalidCACertSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: openvpn-pki-ca
  namespace: d8-openvpn
type: Opaque
data:
  tls.crt: aW52YWxpZCBkYXRhCg==
`
		expiredCertSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: openvpn-pki-ca
  namespace: d8-openvpn
type: Opaque
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURDRENDQWZDZ0F3SUJBZ0lCQVRBTkJna3Foa2lHOXcwQkFRc0ZBREFvTVJJd0VBWURWUVFLRXdsR2JHRnUKZENCS1UwTXhFakFRQmdOVkJBTVRDV1pzWVc1MExtTnZiVEFlRncweU5EQTBNRE14TmpJNE5EaGFGdzB5TkRBMApNREl4TmpJNE5EaGFNQ2d4RWpBUUJnTlZCQW9UQ1Vac1lXNTBJRXBUUXpFU01CQUdBMVVFQXhNSlpteGhiblF1ClkyOXRNSUlCSWpBTkJna3Foa2lHOXcwQkFRRUZBQU9DQVE4QU1JSUJDZ0tDQVFFQTAzcWNEaEtVZGFHWnp2SGcKNFk2ZkVtclMxN0NMKzl1QWdnWDdlbFJLWXZ6Q3pZTXlNbmNhR08zTGs5cUxJVjZOS0JGTDcrd01qYklnSjV5bwpvcCtZVTVwalFkU3owWnVvRVNyWDd4S05GWnh3cVJZME5KTmtoaTRkVERxWnZ1R1JCeTZVbDVaMFNCSjliRFQzCjVHdkhYMjFtTHJDdmVoZDRBYTZQU05VQXFweG85VGw3elZRS3J5Y2NQTUtvdEE0ZlZ0VkFvOHVkSXZIYVphZFUKMnZZSEFUazc3TGMrRHNjUi9YL2lYcUVMdkozR1VkWGxvNXFpWVZwN0pXZzY2RXRrNm5HWnhaZ2sxNEJHaWw3RQp4bEM1WkJyUWFPTFdwOS84S2ppT0U2MEFKZXdmdXpZdklTQ3RSZVZxSzEwUzExQTd6bUtvOGZvdUgxVGRteDlWCkNrM2Myd0lEQVFBQm96MHdPekFPQmdOVkhROEJBZjhFQkFNQ0JhQXdFd1lEVlIwbEJBd3dDZ1lJS3dZQkJRVUgKQXdFd0ZBWURWUjBSQkEwd0M0SUpabXhoYm5RdVkyOXRNQTBHQ1NxR1NJYjNEUUVCQ3dVQUE0SUJBUUI1aW5EcQpYZjR0Nk1rOVQrZHFJR0QwR1pEOE5WNHlOeU9wSmFQaUhIZ0JGVmZMcW1QTlc0aVlQVHVuazc0OVFMOW14dEV6CnRMN1o2bUp0Wnk2Q1NEcEpHRE9pYk5nQ29iaGxqUkhsUkp6S0lZWUxnSHR0a3lYSFNOV1dUeXMzWS81L3ZRTWcKUkxEUmlyUzU4TytvcVp3WTlGZm1lbDNRSU9vVXpBRzU1c2IxRlhLL3Z2MDNMWlhtWnkzZ3d1UEJjbGcrQ0I3eAoweVUyZEF5TmhwM09Jd0hSUWtRUFdHVndUS0IwazFwOHFYcndFdmtnT3FMZ3dhYTdhMThKeTRxZTBkaFpJUFBSCmtvNlNGTksvOWdqY21hZ1BFTGhDTm5kL1pWS1ZhTGtQTzNob2QyOWVUbGg5bkMrbzNRSGlndis5dnc1S3d6WWUKUFlkUnhOZUJILytMcnlpQwotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCgo=
`
		validCACert = `
---
apiVersion: v1
kind: Secret
metadata:
  name: openvpn-pki-ca
  namespace: d8-openvpn
type: Opaque
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUZDVENDQXZHZ0F3SUJBZ0lVZk1xcllHd0t4L01OM054aStCQlljejZkK1RFd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0V6RVJNQThHQTFVRUF3d0lUWGxVWlhOMFEwRXdJQmNOTWpVd05qRTVNRFkxT0RNeFdoZ1BNakV5TlRBMgpNakF3TmpVNE16RmFNQk14RVRBUEJnTlZCQU1NQ0UxNVZHVnpkRU5CTUlJQ0lqQU5CZ2txaGtpRzl3MEJBUUVGCkFBT0NBZzhBTUlJQ0NnS0NBZ0VBeE8vaXBTU0U0U2Q0akxnMTNkOCtqcVkrM0hGMnMzQnRXK2pzNEhjKy8wK04KYkM4WUpuMlMwNHRGNDZXVkROOGd0bnlkOWVlRDVoYkNVdEdTN01iVDhkWWpTenFXeSt4YTRXcEt4OWVoZXBLVgowcjBJcm5vOVNveFppSzVzR2dXYUtUV2xvVnFZejluQjJKdnhUY3o4dnVKVDlsU2F4Q2drYTRqZ0p4UmdTUzlOCjJhQUVBU2tncXlyTkk1NmdxWkV4R1F1ZG1Wa2x6TkRFUE9SSFBzRXVzb2FXb3NwazArWHdLaDFBcEVEZHJLRHIKY2hYWFBpcnRjb21VKzdTV1B5UTd6bis5bmhKUjhiS2k0MFNxTTNqM2lHWDRuU0lVa2I2VGlBa212K210LzI1Ugp1dDhnYXlJaU8xVVliL3NkQmJlOW1LVUc1NElWZkNjYklHc1FhQ0FBeGVlNVI3Vzh6aGlHNUxJb1VtbkZRa3YwClltd3BwOEl2b0dXSng3cWtaaFlFTnVWYytpQ2VPMCt4dFBFdjhiajFjTVZrSHo2dE4yUDFWODIvWkpSNlg4bDkKUk9MZVhZOENNZU9qL0NqaG1BOXY1a01jbEI4aWR0cEhrRE9zRldkZ2NJYUZzTHVubkxLVVhxSVB1bGFyeVBQUAoxR3RpZXV1ZU5zclNFdC9ROWpORCs4cGRyWkl0eE1ybjNoaFBuR2ZQYjFBaW5GZW5jUTUvRitFSTYybXZzYUVIClJ0ckhRazlDY3gvdjEzdjlwUGs5V1VxbWZlRHpyS2tXR0x6MjZnbHJjdkFGTlNjckVucVlVOXJFcVRKcFp6UGoKbzBLb1czWWM5V0tSZW5MR3hFcWhQeU5YNlQxZ3BmVnhQV0FMZlowSW5SNnBhUkNONTc2cm04djhxTStxbUhjQwpBd0VBQWFOVE1GRXdIUVlEVlIwT0JCWUVGSDd5dm1KVXNISUU5QVgwZW1KcFNVOGd6MStPTUI4R0ExVWRJd1FZCk1CYUFGSDd5dm1KVXNISUU5QVgwZW1KcFNVOGd6MStPTUE4R0ExVWRFd0VCL3dRRk1BTUJBZjh3RFFZSktvWkkKaHZjTkFRRUxCUUFEZ2dJQkFNSDkyN3RtOGNGeHFXeDg0VHJzSGphUlhtVW9qRUN5M3B1UjNKUEFRRWpXZ1ZhRwo3RnNWOU9Jcmh6ZSsyS0dSQm1OQnlHT0NZaGNucERaYXhPcXkyQThMcUhMOHNUd1ErU2JNcVNFVjZVVENJcWNYCktHSStIMzYrL2t4VFJNd0c4MDVVd055MWJ6VEpqMWZ4emxBWDE0VFBzaFZyRnNTekxqWG1lL1J5Vk5Ham4zbWUKUG9JTGY3ckIyNFB1R3lrNlQ3eGY5enFxWk9SY0t5akI5M1J2QjlPUlYySjVkSi9TR3d5bnJpODdCazlzV093MwpsZzM4R2hTSUprR0dEV0Z1eUpiTU83dHIxOHFKUkR4NnNpL1RmYlIwUjN6bzFVRnNWTlRYamJVdGt5ZndwaUhZCkg5V1ZLaHJ3VjNzUk9yRkRBZ2pRb2p5ZmJQYjJPNExZZ3dNaktGRHJpeXJEMTVHNnVBV2J5S0xPSDZZa1h0eDIKRFNSZGl3UlY5eXdrK0xmSTlBVDYvU25FaitKeXIrdnBGWVR1ck9nTGs1Q0tMYk9DRFIyRldIS2cwNk1nNzdlbgpmSTlKVkM2Vzl3K3JFdnFMcFhlbC91Y2hveTZGYWpWZWVSMjAwUlNvSUNzRnFKcVNwWkR6SFVnQmZkL1NEenhVClNzbVRWL3NCUy9QMy94L21VRzlNeVJyVmRUcVpzTXpsM25pb3d0ZTU0bFRaQzVRRXJESEpUei9oMk5Gbk1YVjcKTFREYWpMcU5sYzkvR083QjJ5VExtSkVMb0tObDBENkZTTDA4RVdGUzVETXFEYUQyY1hYSE10U01weTY4TDJZKwpmSkZLdVRLNHNpcE9lZ0hyNHBCMnplRDZWeVlYdzFxUS96cUhpcDJyd0lUSzlMMGJCckh4RnAyMnV0dzgKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
`
		validCertSecret = `
---
apiVersion: v1
kind: Secret
metadata:
  name: openvpn-pki-server
  namespace: d8-openvpn
type: Opaque
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSURDekNDQWZPZ0F3SUJBZ0lVTlJLbXJNVEE2bmdUdmtQTGJBUG54M2RLR0tJd0RRWUpLb1pJaHZjTkFRRUwKQlFBd0ZERVNNQkFHQTFVRUF3d0pabXhoYm5RdVkyOXRNQ0FYRFRJMU1EUXdNekUwTURJek9Wb1lEekl4TWpVdwpNekV3TVRRd01qTTVXakFVTVJJd0VBWURWUVFEREFsbWJHRnVkQzVqYjIwd2dnRWlNQTBHQ1NxR1NJYjNEUUVCCkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDNFkzRURERmNWZkhmUXJKcG5NejBRWTRuazVmSFM5OG9XZUpmZUNkSlQKTk9OUTVGd1hCSkM1clBrYXZTRnZIWWprNmEveHROTXM1eVc0bm9wS1VnR1BZU2dUNEd4TWdRZW5YV2F1NmhvVApuT2NsakhVR2o3TlVSS1FKL3lPY1ZKVDl0OW5pYS9ZNHVoOHMxK2VHQ0tOOWZTeDBVbHBEZWtkVTNWQjRuR2VGCmU4VUR0TGlWNm1CbTRndzArUmwrVkQxUkVJYU5iWExHcVMxWUtIcWcvZGhpRHpHVUxRakdhOFZyTU1TbkRHL00KRW8yVjA5VHN2TXh4b04waE9MZUROTjlUZVl6TnVERFRWRkJhT3U0Nk11NmRmUVdLb29leXNET1FpdzRqeGptMgpBU1JqUXdPbisrakxJWlBEUmI0UEhZZ0xBcnlkdkpXNFpUTzErVmh0aDg2WkFnTUJBQUdqVXpCUk1CMEdBMVVkCkRnUVdCQlNONUdYNi8wNE1ITEZUa0hzUmxTZWovMWxKR2pBZkJnTlZIU01FR0RBV2dCU041R1g2LzA0TUhMRlQKa0hzUmxTZWovMWxKR2pBUEJnTlZIUk1CQWY4RUJUQURBUUgvTUEwR0NTcUdTSWIzRFFFQkN3VUFBNElCQVFCOApLd1FmcXQybENMY0xpRUlEZE9hWmNMZ2tRcGZLKzN6aS90bTdDaUNyL0oxSGZkZTZYS29ONTFKQVcwQ1RZdmQyCnNDWWtkTUx6TkR3QlZTZ0lxd3luc3NRMzZTSzFyQWl6YnF2cjZWNUYxd1cwWVMraEhUSWM2cDA3TUpUMFVUdXkKVUg5Vkx5RWR5UnNETDBIbTNCcXR1YU8wcjlGVWw0N3VJVENMa0xVYXI0cTJtYlBFM1YwcFNzWlBXM2tlL0NvWQpEU2U2TUtpVFlHbnBDU3Q1NkY0ZHlnVTBMMVU4SWZsS3dnamx1NEhsazJIRHpMNndoelhCWlVycEFqM3c4dEF2CjJmN2xFZ1RhdDVpV1lNTi9iVDA2R1NsR1hHS0ExTmpQM3g2KzkzV1QzWjdSZzM0ZlRYSDNibFZNYUEzd0hnamwKQjkxeEREMVFWbzVsQkVnU0g0K3AKLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo=
`

		openvpnStatefulSet = `
---
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: openvpn
  namespace: d8-openvpn
spec:
  serviceName: openvpn
  replicas: 1
  selector:
    matchLabels:
      app: openvpn
  template:
    metadata:
      labels:
        app: openvpn
    spec:
      containers:
      - name: openvpn
        image: openvpn:latest
`
	)

	f := HookExecutionConfigInit(`{}`, `{}`)

	Context("An empty cluster", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.GenerateOnStartupContext())
			f.KubeStateSet(``)
			f.RunGoHook()
		})

		It("Hook is executed successfully", func() {
			Expect(f).To(ExecuteSuccessfully())
		})
	})

	Context("Cluster with expired cert", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8OpenvpnNamespace + expiredCertSecret + emptySecret + openvpnStatefulSet))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.RunGoHook()
		})

		// Check cert is deleted
		It("Server cert should be removed", func() {
			Expect(f).To(ExecuteSuccessfully())
			secret := f.KubernetesResource("Secret", "d8-openvpn", "openvpn-pki-server")
			secretCa := f.KubernetesResource("Secret", "d8-openvpn", "openvpn-pki-ca")
			Expect(secret.Exists()).To(BeFalse())
			Expect(secretCa.Exists()).To(BeFalse())
		})
	})

	Context("Cluster with valid cert", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8OpenvpnNamespace + validCertSecret + validCACert + openvpnStatefulSet))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.RunGoHook()
		})

		It("Server cert should exist", func() {
			Expect(f).To(ExecuteSuccessfully())
			secret := f.KubernetesResource("Secret", "d8-openvpn", "openvpn-pki-server")
			Expect(secret.Exists()).To(BeTrue())
		})
	})

	Context("Cluster with empty secret (no cert data)", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8OpenvpnNamespace + emptySecret + validCACert + openvpnStatefulSet))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.RunGoHook()
		})

		It("Hook does not delete empty secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			secret := f.KubernetesResource("Secret", "d8-openvpn", "openvpn-pki-server")
			Expect(secret.Exists()).To(BeTrue())
		})
	})

	Context("Cluster with invalid cert", func() {
		BeforeEach(func() {
			f.BindingContexts.Set(f.KubeStateSet(d8OpenvpnNamespace + invalidCACertSecret + emptySecret + openvpnStatefulSet))
			f.BindingContexts.Set(f.GenerateScheduleContext("*/10 * * * *"))
			f.RunGoHook()
		})

		// Check secret is not deleted
		It("Hook does not delete invalid certificate secret", func() {
			Expect(f).To(ExecuteSuccessfully())
			secret := f.KubernetesResource("Secret", "d8-openvpn", "openvpn-pki-server")
			Expect(secret.Exists()).To(BeTrue())
		})
	})
})
