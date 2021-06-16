package certificate

import (
	"bytes"
	"log"
	"os"

	"github.com/cloudflare/cfssl/csr"
	"github.com/sirupsen/logrus"
)

func GenerateCSR(logger *logrus.Entry, cn string, groups ...string) (csrPEM, key []byte, err error) {
	request := csr.CertificateRequest{
		CN:         cn,
		KeyRequest: csr.NewKeyRequest(),
	}

	for _, group := range groups {
		request.Names = append(request.Names, csr.Name{O: group})
	}
	g := &csr.Generator{Validator: Validator}

	// Catch cfssl logs message
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	csrPEM, key, err = g.ProcessRequest(&request)
	if err != nil {
		logger.Errorln(buf.String())
	}

	return
}

// Validator does nothing and will never return an error. It exists because creating a
// csr.Generator requires a Validator.
func Validator(req *csr.CertificateRequest) error {
	return nil
}
