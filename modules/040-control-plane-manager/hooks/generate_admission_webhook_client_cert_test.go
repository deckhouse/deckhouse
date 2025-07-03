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
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/deckhouse/deckhouse/testing/hooks"
)

const initValues = `
{
  "global": {
    "internal": {
      "modules": {
        "admissionWebhookClientCA": {
          "cert": "-----BEGIN CERTIFICATE-----\nMIIB0jCCAXigAwIBAgIUJNyXgo++IThoPb7bX7Bs4J9hvMswCgYIKoZIzj0EAwIw\nIjEgMB4GA1UEAxMXZGVja2hvdXNlLmQ4LXN5c3RlbS5zdmMwHhcNMjUwNTE5MTIz\nNTAwWhcNMzUwNTE3MTIzNTAwWjAiMSAwHgYDVQQDExdkZWNraG91c2UuZDgtc3lz\ndGVtLnN2YzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABGCaPJqM8LnUz4HNeq+Z\nSw+8POfVItSl8DtW4A2hzkDA3dvW+KLmge1QqfdnCKxt4zs2wtuAj8m7J8xx9FTL\ndNmjgYswgYgwDgYDVR0PAQH/BAQDAgWgMAwGA1UdEwEB/wQCMAAwHQYDVR0OBBYE\nFFS3nR9J5aWEfgH3q/UdAbaqda4YMEkGA1UdEQRCMECCF2RlY2tob3VzZS5kOC1z\neXN0ZW0uc3ZjgiVkZWNraG91c2UuZDgtc3lzdGVtLnN2Yy5jbHVzdGVyLmxvY2Fs\nMAoGCCqGSM49BAMCA0gAMEUCIHaluNHlo5Zyd8zLRtT7DTbDvdh0UzCAlAeei7ku\n+232AiEA8rLlxHNTjX871utesgR0z/Nh9DzZVyL6Jsj/zbZw/HY=\n-----END CERTIFICATE-----",
          "key": "-----BEGIN EC PRIVATE KEY-----\nMHcCAQEEIBTRQbQRVgtt7PPKRiyXaC/FhycAuJfF4c0obaACdPy5oAoGCCqGSM49\nAwEHoUQDQgAEYJo8mozwudTPgc16r5lLD7w859Ui1KXwO1bgDaHOQMDd29b4ouaB\n7VCp92cIrG3jOzbC24CPybsnzHH0VMt02Q==\n-----END EC PRIVATE KEY-----"
        }
      }
    }
  },
  "controlPlaneManager": {
    "internal": {
      "admissionWebhookClientCertificateData": {
        "cert": "",
        "key": ""
      }
    }
  }
}
`

const kubeState = `
---
apiVersion: v1
kind: Secret
metadata:
  name: admission-webhook-client-key-pair
  namespace: d8-system
type: kubernetes.io/tls
data:
  tls.crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUNpekNDQWpHZ0F3SUJBZ0lVYWRRcFNTOG5BeXY0aDd0WGEwRWl5MDEvTmpvd0NnWUlLb1pJemowRUF3SXcKSWpFZ01CNEdBMVVFQXhNWFpHVmphMmh2ZFhObExtUTRMWE41YzNSbGJTNXpkbU13SGhjTk1qVXdOakF5TVRBdwpOakF3V2hjTk16VXdOVE14TVRBd05qQXdXakFkTVJzd0dRWURWUVFERXhKcmRXSmxMV0Z3YVMxaFpHMXBjM05wCmIyNHdnZ0VpTUEwR0NTcUdTSWIzRFFFQkFRVUFBNElCRHdBd2dnRUtBb0lCQVFDYnBwSWJvL0JYWUluUThtbVYKc0tzeUhySHNJajg1YVQ4QnF6V2loTHVtdWgvS2xha2sxaEl4S3pYcUtyVnBZNFFjdHE5dGFyNElTZWpUbWttLwpqa0pZK1pDWWVmMHZXMWxwVzNGaXRXUVFwQm45eE1YeXN4UHBKa1RHRkowTlBMcStFY0RScUNpbUROY25qTEljCmx3ZGZGVHUrYmJZRGRYT2FTNXBlYzR6VUQ2ZDhwaCszM1RpZHM2b3JZUS84WUM4MGxETUlXbzFwN2wxREFYbGgKanVRUGE2d3lUdDUzZFZGNmlWT0hLSnZQdEIwaC9sTHZkZEE0N2JtekNraVB1YzM3anpsNFBISisveGF5WjY3aQoxRW1mUzhGWnZacVptVVBkTVZ4d250UWk2SVpGa0Y1WmJGbkdMYWZDZHU4VUY2Q2pTUWV3U2l6RkpnMEpwS3FhCkJEZDNBZ01CQUFHamZ6QjlNQTRHQTFVZER3RUIvd1FFQXdJRm9EQWRCZ05WSFNVRUZqQVVCZ2dyQmdFRkJRY0QKQVFZSUt3WUJCUVVIQXdJd0RBWURWUjBUQVFIL0JBSXdBREFkQmdOVkhRNEVGZ1FVdjh0TnpvNXhZY1d1akxZVApPV2lWSmt0Zks1Y3dId1lEVlIwakJCZ3dGb0FVVkxlZEgwbmxwWVIrQWZlcjlSMEJ0cXAxcmhnd0NnWUlLb1pJCnpqMEVBd0lEU0FBd1JRSWhBTm5zU0F3U1FDbHFDSVBTSmljbllNQURaRjY4REpUVTh3cm4zR05RNnpkc0FpQWYKay9NUHBZb21zL2d2dmRUTFk1T2lia0FTNm1VUURJbjAwYUMzUVhUK2VBPT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQoK
  tls.key: LS0tLS1CRUdJTiBSU0EgUFJJVkFURSBLRVktLS0tLQpNSUlFb3dJQkFBS0NBUUVBbTZhU0c2UHdWMkNKMFBKcGxiQ3JNaDZ4N0NJL09Xay9BYXMxb29TN3Byb2Z5cFdwCkpOWVNNU3MxNmlxMWFXT0VITGF2YldxK0NFbm8wNXBKdjQ1Q1dQbVFtSG45TDF0WmFWdHhZclZrRUtRWi9jVEYKOHJNVDZTWkV4aFNkRFR5NnZoSEEwYWdvcGd6WEo0eXlISmNIWHhVN3ZtMjJBM1Z6bWt1YVhuT00xQStuZktZZgp0OTA0bmJPcUsyRVAvR0F2TkpRekNGcU5hZTVkUXdGNVlZN2tEMnVzTWs3ZWQzVlJlb2xUaHlpYno3UWRJZjVTCjczWFFPTzI1c3dwSWo3bk4rNDg1ZUR4eWZ2OFdzbWV1NHRSSm4wdkJXYjJhbVpsRDNURmNjSjdVSXVpR1JaQmUKV1d4WnhpMm53bmJ2RkJlZ28wa0hzRW9zeFNZTkNhU3FtZ1EzZHdJREFRQUJBb0lCQUNwKzl1TXZCajZFNy91cApRNlNINEtYRmRhUlgwVlUyWStwcnVUQU85MERWRGpyOFUzcm9LMzFiSTlRMEp1V0lNeGkzMC91V3FoMlBPRThWCmo4OEo0eGx4d2NXdmdLMitUaElTMGtrMTV3VVlHTFNOVmRJbWpHZ2FkNnB4RkZQNTRhNWxJSFRkYVlPMGU4V2oKaHptQkRwVXFNZTZTKzUvRUlIVGU4VjJhUkxmY3hVZnRPYU1mTVNYTllVUXRZaEMwVC9CbHBnYkxqaWZ2M01nRQoyakFWcUQwKytRQVo1R2tnRy9nL3Z0V2QrVUQyVWVENGdRRno3Ykp1ZHdacXRrS0lxVjFYRkZwaXRCa1VCZGRLCk10T3dkZHpIaUlFblVCWk81TzRQZlMrN1NWaE9RMmxJN1dOTzBOOVFqS2dXa0gzeUtqRFI3RWlFcEFxN0s2dkIKN3ZXekRua0NnWUVBeGxWQ0pIODUxUlN5QUQ0dGN1bkNBMnBIdEk3NE1zeGM1dmExSmpVczdvZ3dWQUVzeXJoUwphQjFwdUgrODlVMUlFSjFpczljUytPY3dPZWpzb00yVmIyWXRUbjBWTGcvL1hFVzJqN2JlOE00WElhWXdmY3JqCmpaQmFHVkNwWUg4alpCMWNuUkZsT3MwbzJtTXp4b0RYS3dFQld1aFJCc3FKYXd3RGJDaXUxWE1DZ1lFQXlPaEoKeW54WHVtVUxkcGt4d0FuWGs3R2NRRUtCNnFBZFdTdU4xYjRQdnJDYS9DNUptYzQxYzlNTVRjc0w3QjJCTW9PUwpnaEVCSmg1Y1NmN3hLaERDK3pneFlMSFQ2Nmo4RFdxbE4yanE1UTI3NVIvdWNRTGNMbE1TcGUxOWcwVGp6RWJtCmxoV1hmanFLUHg2L2FLb2EvSitMc0xIdFErc3pLd3FHd0RxdDlPMENnWUVBdUdmM2FzWGNlTkdZTzMrVWRIOEMKUkhpaUdUREJkMEhxczFqNXozK3J1bEZvVmdNTnFhTStBODR0U1QyRDdMU2haOGxlUjRhVy9sUyswMmxOOHFtVAo0eE1tMXc2WURjOFVDTEJNOFV6Lzl2ZzRLN0pBN2dVaUZMTCtBd0dycXF3cnROOVVDRHB2Vy8vN2x5cWJybHFICk5WWG04NmFFQ0FOelI5UFFydFVZMGg4Q2dZQWsvUzlveWsvVWozbjIwZW1vODZidkdFb3VRcEJzeENIakl2T3cKSUpnQmdiNW5JNWFGYk1QR21WcXdqK3VZQXk2Z1FEMGZHVVplNEVRWms0aVBPQnJONmVDZGJ1QVhpVHN1dFMzSgo3OVVmYXRIbk0yUFJCcmZIQjZCdFVEWkZqczlwOHJ2TmNoZzhNMGIwckJLTmtKUDdZdHh6SWE4UFRDUlZqbENVCjM3amJXUUtCZ0JZNFpJdFNYRTV4V0xpVFVSK3o2RGZlOGxhN1c1UGUwWjlBYU9Xb3BYY3N5UG82UXZ0OXNBSG4KTU9ibEtFa0xoTEthdmZKazhZTTV4alhjQkFGdCtoYVdSNkpQTEsvdGRjMlVnREsvcHBmVjdLU1NkSmdDUWZpMwpCajRvWmREMTltWmoyKytuYy9YZlV1eTd5SFJGUDFWUGlhRHNUUU85L0d5Y1FhY2JBZFMrCi0tLS0tRU5EIFJTQSBQUklWQVRFIEtFWS0tLS0tCgo=
`

const expectedCertCN = "kube-api-admission"
const expectedIssuerCN = "deckhouse.d8-system.svc"

var _ = Describe("control-plane-manager :: hooks :: generate_admission_webhook_cert ::", func() {
	executor := HookExecutionConfigInit(initValues, "")

	Context("Generate new certificate", func() {

		BeforeEach(func() {
			executor.KubeStateSet("")
			executor.BindingContexts.Set(executor.GenerateBeforeHelmContext())
			executor.RunHook()
		})

		It("Must be executed successfully", func() {
			cert := executor.ValuesGet("controlPlaneManager.internal.admissionWebhookClientCertificateData.cert").String()
			key := executor.ValuesGet("controlPlaneManager.internal.admissionWebhookClientCertificateData.key").String()

			result, err := validateCertificate([]byte(cert), []byte(key), expectedCertCN, expectedIssuerCN)
			if err != nil {
				Fail(err.Error())
			}
			Expect(result).To(BeTrue())
		})
	})

	Context("Reuse certificate from Secret", func() {

		BeforeEach(func() {
			executor.KubeStateSet(kubeState)
			executor.BindingContexts.Set(executor.GenerateBeforeHelmContext())
			executor.RunHook()
		})

		It("Must be executed successfully", func() {
			cert := executor.ValuesGet("controlPlaneManager.internal.admissionWebhookClientCertificateData.cert").String()
			key := executor.ValuesGet("controlPlaneManager.internal.admissionWebhookClientCertificateData.key").String()

			result, err := validateCertificate([]byte(cert), []byte(key), expectedCertCN, expectedIssuerCN)
			if err != nil {
				Fail(err.Error())
			}
			Expect(result).To(BeTrue())
		})
	})
})

func validateCertificate(certBytes []byte, keyBytes []byte, expectedCertCN string, expectedIssuerCN string) (bool, error) {
	// Try parse certificate
	block, _ := pem.Decode(certBytes)
	if block == nil || block.Type != "CERTIFICATE" {
		return false, fmt.Errorf("failed to decode certificate")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("failed to parse certificate: %s", err.Error())
	}

	// Checking the compatibility of the key and certificate
	_, err = tls.X509KeyPair(certBytes, keyBytes)
	if err != nil {
		return false, err
	}

	// Check certificate common name (CN)
	if cert.Subject.CommonName != expectedCertCN {
		return false, fmt.Errorf("certificate common name (%s) not equal to expected common name (%s)", cert.Subject.CommonName, commonName)
	}

	// Check certificate issuer common name (CN)
	if cert.Issuer.CommonName != expectedIssuerCN {
		return false, fmt.Errorf("certificate issuer common name (%s) not equal to expected common name (%s)", cert.Issuer.CommonName, expectedIssuerCN)
	}

	return true, nil
}
