package sender

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	log "github.com/sirupsen/logrus"

	"upmeter/pkg/app"
	"upmeter/pkg/probe/types"
)

type UpmeterClient struct {
	Ip   string
	Port string

	httpClient *http.Client
}

func CreateUpmeterClient(ip string, port string) *UpmeterClient {
	return &UpmeterClient{
		Ip:   ip,
		Port: port,
	}
}

func (c *UpmeterClient) Send(results []types.DowntimeEpisode) error {
	// encode to JSON
	jsonBytes, err := json.Marshal(results)
	if err != nil {
		return fmt.Errorf("encode probe results to JSON: %v", err)
	}

	// log.Infof("Send body: %s", string(jsonBytes))
	// send over http
	// return error if something goes wrong
	url := fmt.Sprintf("%s://%s:%s/downtime", c.Schema(), c.Ip, c.Port)
	req, err := http.NewRequest("POST", url, bytes.NewReader(jsonBytes))
	if err != nil {
		log.Errorf("Create POST request: %v", err)
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.HttpClient().Do(req)
	if err != nil {
		log.Errorf("Send %d results to upmeter: %v", len(results), err)
		return err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Send %d results to upmeter read body: %v", len(results), err)
		return err
	}

	if resp.StatusCode != 200 {
		log.Errorf("Send results to upmeter failed with code %d: %v", resp.StatusCode, string(body))
		return fmt.Errorf("send results failed with code %d", resp.StatusCode)
	}

	return nil
}

func (c *UpmeterClient) HttpClient() *http.Client {
	createHttpClient := func() *http.Client {
		if app.UpmeterTls == "false" {
			return http.DefaultClient
		}

		tlsTransport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		}

		if app.UpmeterCaPath != "" {
			// Create transport with tls and CA certificate checking
			caCertBytes, err := ioutil.ReadFile(app.UpmeterCaPath)
			if err != nil {
				log.Errorf("Fallback to default http client. Cannot read CA certificate from '%s': %v", app.UpmeterCaPath, err)
				return http.DefaultClient
			}
			caCertPool := x509.NewCertPool()
			caCertPool.AppendCertsFromPEM(caCertBytes)

			tlsTransport = &http.Transport{
				TLSClientConfig: &tls.Config{
					RootCAs: caCertPool,
				},
			}
		}

		// Wrap tls transport to add Authorization header.
		bearerBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/token")
		if err != nil {
			log.Errorf("Fallback to default http client. Cannot read Kubernetes token: %v", err)
			return http.DefaultClient
		}

		kbt := NewKubeBearerTransport(tlsTransport, string(bearerBytes))

		// Create https client with checking CA certificate and Authorization header
		client := &http.Client{
			Transport: kbt,
		}

		return client
	}
	if c.httpClient == nil {
		c.httpClient = createHttpClient()
	}
	return c.httpClient
}

func (c *UpmeterClient) Schema() string {
	if app.UpmeterTls == "false" {
		return "http"
	}
	return "https"
}

func NewKubeBearerTransport(next http.RoundTripper, bearer string) *KubeBearerTransport {
	return &KubeBearerTransport{
		next:        next,
		bearerToken: bearer,
	}
}

type KubeBearerTransport struct {
	next        http.RoundTripper
	bearerToken string
}

func (t *KubeBearerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Add("Authorization", "Bearer "+t.bearerToken)
	return t.next.RoundTrip(req)
}
