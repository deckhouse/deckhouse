package main

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

var (
	listenHost              = "0.0.0.0"
	listenPort              = "80"
	serviceAccountTokenPath = "/var/run/secrets/kubernetes.io/serviceaccount/token"
)

func main() {
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/error", errorHandler)
	http.HandleFunc("/api", apiHandler)
	http.HandleFunc("/disk", diskHandler)
	http.HandleFunc("/dns", dnsHandler)
	http.HandleFunc("/neighbor", neighborHandler)
	http.HandleFunc("/prometheus", prometheusHandler)

	log.Fatalln(http.ListenAndServe(listenHost+":"+listenPort, nil))
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(os.Getenv("HOSTNAME"), r.RemoteAddr, r.RequestURI)
	if r.RequestURI != "/" {
		w.WriteHeader(404)
		fmt.Fprintf(w, "404 Not Found %s\n", r.RequestURI)
		return
	}
	fmt.Fprintf(w, "ok")
}

func errorHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.RequestURI)
	w.WriteHeader(500)
	fmt.Fprintf(w, "ok")
}

func apiHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.RequestURI)

	apiserverEndpoint := "https://127.0.0.1:6445/readyz/ping"

	kubernetesServiceHost := os.Getenv("KUBERNETES_SERVICE_HOST")
	kubernetesServicePort := os.Getenv("KUBERNETES_SERVICE_PORT_HTTPS")
	namespace := "d8-smoke-mini"
	podName := os.Getenv("HOSTNAME")
	if kubernetesServiceHost != "" && kubernetesServicePort != "" {
		apiserverEndpoint = fmt.Sprintf("https://%s:%s/api/v1/namespaces/%s/pods/%s", kubernetesServiceHost, kubernetesServicePort, namespace, podName)
	}

	serviceaccountToken, err := ioutil.ReadFile(serviceAccountTokenPath)
	if err != nil {
		w.WriteHeader(500)
		log.Println(err)
		return
	}

	bearer := fmt.Sprintf("Bearer %s", string(serviceaccountToken))
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	req, nil := http.NewRequest("GET", apiserverEndpoint, nil)
	req.Header.Add("Authorization", bearer)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		w.WriteHeader(500)
		return
	}
	defer resp.Body.Close()
	log.Println(resp.StatusCode, resp.Request.URL)
	if resp.StatusCode != 200 {
		w.WriteHeader(500)
		return
	}
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		w.WriteHeader(500)
		return
	}
	fmt.Fprintf(w, "ok")
}

func diskHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.RequestURI)
	originalContent := string(time.Now().UnixNano())
	tmpFilePath := fmt.Sprintf("/disk/sm-%s", string(time.Now().UnixNano()))
	err := ioutil.WriteFile(tmpFilePath, []byte(originalContent), 0644)
	if err != nil {
		log.Println(err)
		w.WriteHeader(500)
		return
	}
	content, err := ioutil.ReadFile(tmpFilePath)
	if err != nil {
		w.WriteHeader(500)
		log.Println(err)
		return
	}
	err = os.Remove(tmpFilePath)
	if err != nil {
		w.WriteHeader(500)
		log.Println(err)
		return
	}
	if originalContent == string(content) {
		fmt.Fprintf(w, "ok")
	} else {
		w.WriteHeader(500)
	}
}

func dnsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.RequestURI)
	_, err := net.LookupIP("kubernetes.default")
	if err != nil {
		w.WriteHeader(500)
		log.Println(err)
		return
	}
	fmt.Fprintf(w, "ok")
}

func prometheusHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.RequestURI)
	prometheusEndpoint := "https://prometheus.d8-monitoring:9090/api/v1/metadata?metric=prometheus_build_info"

	serviceaccountToken, err := ioutil.ReadFile(serviceAccountTokenPath)
	if err != nil {
		w.WriteHeader(500)
		log.Println(err)
		return
	}

	bearer := fmt.Sprintf("Bearer %s", string(serviceaccountToken))
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	req, nil := http.NewRequest("GET", prometheusEndpoint, nil)
	req.Header.Add("Authorization", bearer)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
		w.WriteHeader(500)
		return
	}
	defer resp.Body.Close()
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		w.WriteHeader(500)
		return
	}
	fmt.Fprintf(w, "ok")
}

func neighborHandler(w http.ResponseWriter, r *http.Request) {
	log.Println(r.RemoteAddr, r.RequestURI)
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
			resp, err := client.Get(fmt.Sprintf("http://smoke-mini-%s/", targetServices[i]))
			if err != nil {
				log.Println(err)
				errorCount++
				continue
			}
			defer resp.Body.Close()
			body, err := ioutil.ReadAll(resp.Body)
			if err != nil || string(body) != "ok" {
				log.Println(err)
				errorCount++
			}
		} else {
			w.WriteHeader(500)
			return
		}
	}
	fmt.Fprintf(w, "ok")
}
