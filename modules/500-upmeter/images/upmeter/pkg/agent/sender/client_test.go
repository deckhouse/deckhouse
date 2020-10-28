package sender

import (
	"fmt"
	"net/http"
	"testing"

	. "github.com/onsi/gomega"

	"upmeter/pkg/app"
)

// CA certificate checking
func Test_google_com_CA(t *testing.T) {
	t.SkipNow()
	g := NewWithT(t)

	cl := CreateUpmeterClient("google.com", "443")
	app.UpmeterCaPath = "testdata/GlobalSign_Root_CA_-_R2.crt"
	schema := cl.Schema()
	g.Expect(schema).Should(Equal("https"))

	url := fmt.Sprintf("%s://google.com/", schema)

	resp, err := cl.HttpClient().Get(url)
	g.Expect(err).ShouldNot(HaveOccurred())

	g.Expect(resp.StatusCode).Should(Equal(http.StatusOK))

	url = fmt.Sprintf("%s://yandex.ru/", schema)

	resp, err = cl.HttpClient().Get(url)
	g.Expect(err).Should(HaveOccurred())
}
