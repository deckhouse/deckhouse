package main

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"
)

var (
	listenHost              = "0.0.0.0"
	listenPort              = "8080"
	serviceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	ready                   = true
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
}

func main() {
	s := &http.Server{
		Handler: setupHandlers(),
		Addr:    listenHost + ":" + listenPort,
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		ready = false
		log.Info(sig)
		s.Shutdown(context.TODO())
	}()

	err := s.ListenAndServe()
	if err == nil || err == http.ErrServerClosed {
		log.Info("Shutdown.")
		return
	}
	log.Fatal(err)
}

func setupHandlers() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/ready", readyHandler)
	mux.HandleFunc("/", rootHandler)
	mux.HandleFunc("/error", errorHandler)
	mux.HandleFunc("/api", apiHandler)
	mux.HandleFunc("/disk", diskHandler)
	mux.HandleFunc("/dns", dnsHandler)
	mux.HandleFunc("/neighbor", neighborHandler)
	mux.HandleFunc("/neighbor-via-service", neighborViaServiceHandler)
	mux.HandleFunc("/prometheus", prometheusHandler)

	return mux
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	if ready {
		w.WriteHeader(200)
	} else {
		w.WriteHeader(500)
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	log.Info(os.Getenv("HOSTNAME"), r.RemoteAddr, r.RequestURI)
	if r.RequestURI != "/" {
		w.WriteHeader(404)
		fmt.Fprintf(w, "404 Not Found %s\n", r.RequestURI)
		return
	}
	fmt.Fprintf(w, "ok")
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
	log.Info(r.RemoteAddr, r.RequestURI)
	w.WriteHeader(500)
	fmt.Fprintf(w, "ok")
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
	log.Info(r.RemoteAddr, r.RequestURI)

	apiserverEndpoint := "https://127.0.0.1:6445/readyz/ping"

	kubernetesServiceHost := os.Getenv("KUBERNETES_SERVICE_HOST")
	kubernetesServicePort := os.Getenv("KUBERNETES_SERVICE_PORT_HTTPS")
	namespace := os.Getenv("POD_NAMESPACE")
	podName := os.Getenv("HOSTNAME")
	if kubernetesServiceHost != "" && kubernetesServicePort != "" {
		apiserverEndpoint = fmt.Sprintf("https://%s:%s/api/v1/namespaces/%s/pods/%s", kubernetesServiceHost, kubernetesServicePort, namespace, podName)
	}

	serviceaccountToken, err := ioutil.ReadFile(serviceAccountTokenPath)
	if err != nil {
		w.WriteHeader(500)
		log.Error(err)
		return
	}

	bearer := fmt.Sprintf("Bearer %s", string(serviceaccountToken))
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	req, nil := http.NewRequest("GET", apiserverEndpoint, nil)
	req.Header.Add("Authorization", bearer)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error(err)
		w.WriteHeader(500)
		return
	}
	defer resp.Body.Close()
	log.Info(resp.StatusCode, resp.Request.URL)
	if resp.StatusCode != 200 {
		w.WriteHeader(500)
		return
	}
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		w.WriteHeader(500)
		return
	}
	fmt.Fprintf(w, "ok")
}

func diskHandler(w http.ResponseWriter, r *http.Request) {
	log.Info(r.RemoteAddr, r.RequestURI)
	originalContent := fmt.Sprint(time.Now().UnixNano())
	tmpFilePath := fmt.Sprintf("/disk/sm-%s", originalContent)
	err := ioutil.WriteFile(tmpFilePath, []byte(originalContent), 0o644)
	if err != nil {
		log.Error(err)
		w.WriteHeader(500)
		return
	}
	content, err := ioutil.ReadFile(tmpFilePath)
	if err != nil {
		w.WriteHeader(500)
		log.Error(err)
		return
	}
	err = os.Remove(tmpFilePath)
	if err != nil {
		w.WriteHeader(500)
		log.Error(err)
		return
	}
	if originalContent == string(content) {
		fmt.Fprintf(w, "ok")
	} else {
		w.WriteHeader(500)
	}
}

func dnsHandler(w http.ResponseWriter, r *http.Request) {
	log.Info(r.RemoteAddr, r.RequestURI)
	_, err := net.LookupIP("kubernetes.default")
	if err != nil {
		w.WriteHeader(500)
		log.Error(err)
		return
	}
	fmt.Fprintf(w, "ok")
}

func prometheusHandler(w http.ResponseWriter, r *http.Request) {
	log.Info(r.RemoteAddr, r.RequestURI)
	prometheusEndpoint := "https://prometheus.d8-monitoring:9090/api/v1/metadata?metric=prometheus_build_info"

	serviceaccountToken, err := ioutil.ReadFile(serviceAccountTokenPath)
	if err != nil {
		w.WriteHeader(500)
		log.Error(err)
		return
	}

	bearer := fmt.Sprintf("Bearer %s", string(serviceaccountToken))
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	req, nil := http.NewRequest("GET", prometheusEndpoint, nil)
	req.Header.Add("Authorization", bearer)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Error(err)
		w.WriteHeader(500)
		return
	}
	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Error(err)
		w.WriteHeader(500)
		return
	}
	fmt.Fprintf(w, "ok")
}

func neighborHandler(w http.ResponseWriter, r *http.Request) {
	log.Info(r.RemoteAddr, r.RequestURI)
	targetServices := strings.Split(os.Getenv("SMOKE_MINI_STS_LIST"), " ")
	for i := len(targetServices) - 1; i >= 0; i-- {
		if fmt.Sprintf("smoke-mini-%s-0", targetServices[i]) == os.Getenv("HOSTNAME") {
			targetServices = append(targetServices[:i], targetServices[i+1:]...)
		}
	}
	client := http.Client{
		Timeout: 2 * time.Second,
	}
	errorCount := 0
	for i := 0; i < len(targetServices); i++ {
		if errorCount <= 2 {
			resp, err := client.Get(fmt.Sprintf("http://smoke-mini-%s:8080/", targetServices[i]))
			if err != nil {
				log.Error(err)
				errorCount++
				continue
			}
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil || string(body) != "ok" {
				log.Error(err)
				errorCount++
			}
		} else {
			w.WriteHeader(500)
			return
		}
	}
	fmt.Fprintf(w, "ok")
}

func neighborViaServiceHandler(w http.ResponseWriter, r *http.Request) {
	log.Info(r.RemoteAddr, r.RequestURI)
	targetsCount := len(strings.Split(os.Getenv("SMOKE_MINI_STS_LIST"), " ")) - 1
	client := http.Client{
		Timeout: 2 * time.Second,
	}
	errorCount := 0
	maxErrors := 2
	for i := 0; i < targetsCount; i++ {
		if errorCount <= maxErrors {
			resp, err := client.Get("http://smoke-mini:8080/")
			if err != nil {
				log.Error(err)
				errorCount++
				continue
			}
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil || string(body) != "ok" {
				log.Error(err)
				errorCount++
			}
		} else {
			w.WriteHeader(500)
			return
		}
	}
	fmt.Fprintf(w, "ok")
}
