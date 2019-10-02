package main

import "fmt"
import "os"
import "io/ioutil"
import "net/http"
import "crypto/tls"
import "crypto/x509"
import "encoding/json"
import "regexp"
import "log"
import "time"

var sslListenCert = "/etc/ssl/user-authz-webhook/webhook-server.crt"
var sslListenKey = "/etc/ssl/user-authz-webhook/webhook-server.key"
var sslClientCA = "/etc/ssl/apiserver-authentication-requestheader-client-ca/ca.crt"

type DirectoryEntry struct {
	AllowAccessToSystemNamespaces bool
	LimitNamespacesAbsent         bool
	LimitNamespaces               []*regexp.Regexp
}

//             [user type] [user name]
var directory map[string]map[string]DirectoryEntry
var appliedConfigMtime int64 = 0

const configPath = "/etc/user-authz-webhook/config.json"

var systemNamespaces = []string{"antiopa", "kube-.*", "d8-.*", "loghouse", "default"}
var systemNamespacesRegex []*regexp.Regexp

var logger = log.New(os.Stdout, "http: ", log.LstdFlags)

type WebhookRequest struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       struct {
		Group              []string `json:"group"`
		ResourceAttributes struct {
			Group     string `json:"group,omitempty"`
			Namespace string `json:"namespace,omitempty"`
			Resource  string `json:"resource"`
			Verb      string `json:"verb"`
		} `json:"resourceAttributes"`
		User string `json:"user"`
	} `json:"spec"`
	Status struct {
		Allowed bool `json:"allowed"`
		Denied  bool `json:"denied,omitempty"`
	} `json:"status"`
}

type UserAuthzConfig struct {
	Crds []struct {
		Name string `json:"name"`
		Spec struct {
			AccessLevel                   string   `json:"accessLevel"`
			PortForwarding                bool     `json:"portForwarding"`
			AllowScale                    bool     `json:"allowScale"`
			AllowAccessToSystemNamespaces bool     `json:"allowAccessToSystemNamespaces"`
			LimitNamespaces               []string `json:"limitNamespaces"`
			AdditionalRoles               []struct {
				APIGroup string `json:"apiGroup"`
				Kind     string `json:"kind"`
				Name     string `json:"name"`
			} `json:"additionalRoles"`
			Subjects []struct {
				Kind      string `json:"kind"`
				Name      string `json:"name"`
				Namespace string `json:"namespace"`
			} `json:"subjects"`
		} `json:"spec,omitempty"`
	} `json:"crds"`
}

func initVars() {
	for _, systemNamespace := range systemNamespaces {
		r, _ := regexp.Compile("^" + systemNamespace + "$")
		systemNamespacesRegex = append(systemNamespacesRegex, r)
	}
}

func http_handler_healthz(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "Ok.")
	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path)
}

func http_handler_main(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Only POST method is supported.", http.StatusMethodNotAllowed)
		return
	}

	fStat, _ := os.Stat(configPath)
	if mtime := fStat.ModTime().Unix(); mtime != appliedConfigMtime {
		appliedConfigMtime = mtime
		var config UserAuthzConfig

		f, _ := os.Open(configPath)
		configRawData, _ := ioutil.ReadAll(f)
		f.Close()

		json.Unmarshal(configRawData, &config)

		directory = map[string]map[string]DirectoryEntry{
			"User":           map[string]DirectoryEntry{},
			"Group":          map[string]DirectoryEntry{},
			"ServiceAccount": map[string]DirectoryEntry{},
		}

		for _, crd := range config.Crds {
			for _, subject := range crd.Spec.Subjects {
				var subjectName = subject.Name
				if subject.Kind == "ServiceAccount" {
					subjectName = "system:serviceaccount:" + subject.Namespace + ":" + subject.Name
				}

				if _, ok := directory[subject.Kind][subjectName]; !ok {
					directory[subject.Kind][subjectName] = DirectoryEntry{}
				}

				dirEntry := directory[subject.Kind][subjectName]

				if len(crd.Spec.LimitNamespaces) > 0 {
					for _, ln := range crd.Spec.LimitNamespaces {
						r, _ := regexp.Compile("^" + ln + "$")
						dirEntry.LimitNamespaces = append(dirEntry.LimitNamespaces, r)
					}
				} else {
					dirEntry.LimitNamespacesAbsent = true
				}

				dirEntry.AllowAccessToSystemNamespaces = dirEntry.AllowAccessToSystemNamespaces || crd.Spec.AllowAccessToSystemNamespaces
				directory[subject.Kind][subjectName] = dirEntry
			}
		}
	}

	var request WebhookRequest
	json.NewDecoder(r.Body).Decode(&request)

	var dirEntriesAffected = []DirectoryEntry{}
	var summaryLimitNamespaces = []*regexp.Regexp{}
	var summaryLimitNamespacesAbsent = false
	var summaryAllowAccessToSystemNamespaces = false

	if dirEntry, ok := directory["User"][request.Spec.User]; ok {
		dirEntriesAffected = append(dirEntriesAffected, dirEntry)
	}

	if dirEntry, ok := directory["ServiceAccount"][request.Spec.User]; ok {
		dirEntriesAffected = append(dirEntriesAffected, dirEntry)
	}

	for _, group := range request.Spec.Group {
		if dirEntry, ok := directory["Group"][group]; ok {
			dirEntriesAffected = append(dirEntriesAffected, dirEntry)
		}
	}

	if len(dirEntriesAffected) > 0 {
		// Our Guy
		for _, dirEntry := range dirEntriesAffected {
			summaryAllowAccessToSystemNamespaces = summaryAllowAccessToSystemNamespaces || dirEntry.AllowAccessToSystemNamespaces
			summaryLimitNamespacesAbsent = summaryLimitNamespacesAbsent || dirEntry.LimitNamespacesAbsent
			summaryLimitNamespaces = append(summaryLimitNamespaces, dirEntry.LimitNamespaces...)
		}

		if len(request.Spec.ResourceAttributes.Namespace) > 0 {
			if summaryLimitNamespacesAbsent {
				request.Status.Denied = false
				if !summaryAllowAccessToSystemNamespaces {
					for _, pattern := range systemNamespacesRegex {
						if pattern.MatchString(request.Spec.ResourceAttributes.Namespace) {
							request.Status.Denied = true
							break
						}
					}

				}
			} else {
				request.Status.Denied = true
				if summaryAllowAccessToSystemNamespaces {
					summaryLimitNamespaces = append(summaryLimitNamespaces, systemNamespacesRegex...)
				}

				for _, pattern := range summaryLimitNamespaces {
					if pattern.MatchString(request.Spec.ResourceAttributes.Namespace) {
						request.Status.Denied = false
						break
					}
				}
			}
		}
	}

	respData, _ := json.Marshal(request)
	fmt.Fprint(w, string(respData))

	logger.Println(r.RemoteAddr, r.Method, r.UserAgent(), r.URL.Path, "User:", request.Spec.User, "Group:", request.Spec.Group, "Namespace:", request.Spec.ResourceAttributes.Namespace, "Denied:", request.Status.Denied)
}

func main() {
	initVars()

	listenAddr := "127.0.0.1:40443"

	logger.Println("Server is starting to listen on ", listenAddr, "...")

	router := http.NewServeMux()
	router.Handle("/healthz", http.HandlerFunc(http_handler_healthz))
	router.Handle("/", http.HandlerFunc(http_handler_main))

	var clientCertBytes []byte
	clientCertPool := x509.NewCertPool()

	// for requests from apiserver
	clientCertBytes, _ = ioutil.ReadFile(sslClientCA)
	clientCertPool.AppendCertsFromPEM(clientCertBytes)

	// for requests from livenessProbe
	clientCertBytes, _ = ioutil.ReadFile(sslListenCert)
	clientCertPool.AppendCertsFromPEM(clientCertBytes)

	tlsCfg := &tls.Config{
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  clientCertPool,
	}

	server := &http.Server{
		Addr:         listenAddr,
		TLSConfig:    tlsCfg,
		Handler:      router,
		ErrorLog:     logger,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  15 * time.Second,
	}

	if err := server.ListenAndServeTLS(sslListenCert, sslListenKey); err != nil && err != http.ErrServerClosed {
		logger.Fatalf("Could not listen on %s: %v\n", listenAddr, err)
	}
}
