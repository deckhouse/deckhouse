package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"
	"html/template"
	"io"
	"net/http"
	"os"
	"path"
	"regexp"
	"strings"
)

// Get some status info
func statusHandler(w http.ResponseWriter, r *http.Request) {
	var msg []string
	status := "ok"

	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	err := updateReleasesStatus()
	if err != nil {
		msg = append(msg, err.Error())
		status = "error"
	}

	_ = json.NewEncoder(w).Encode(
		APIStatusResponseType{
			Status:         status,
			Msg:            strings.Join(msg, " "),
			RootVersion:    getRootReleaseVersion(),
			RootVersionURL: VersionToURL(getRootReleaseVersion()),
			Releases:       ReleasesStatus.Releases,
		})
}

// X-Redirect to the stablest documentation version for specific group
func groupHandler(w http.ResponseWriter, r *http.Request) {
	_ = updateReleasesStatus()
	log.Debugln("Use handler - groupHandler")
	vars := mux.Vars(r)
	if version, err := getVersionFromGroup(&ReleasesStatus, vars["group"]); err == nil {
		w.Header().Set("X-Accel-Redirect", fmt.Sprintf("/%s/documentation/%s/%s", vars["lang"], VersionToURL(version), getDocPageURLRelative(r, true)))
	} else {
		activeRelease := getRootRelease()
		http.Redirect(w, r, fmt.Sprintf("/%s/documentation/%s/", vars["lang"], activeRelease), 302)
	}

}

// Handles request to /v<group>-<channel>/. E.g. /v1.2-beta/
// Temprarily redirect to specific version
func groupChannelHandler(w http.ResponseWriter, r *http.Request) {
	log.Debugln("Use handler - groupChannelHandler")
	pageURLRelative := "/"
	vars := mux.Vars(r)
	_ = updateReleasesStatus()
	var version, URLToRedirect string
	var err error

	re := regexp.MustCompile(`^/(ru|en)/documentation/[^/]+/(.+)$`)
	res := re.FindStringSubmatch(r.URL.RequestURI())
	if res != nil {
		pageURLRelative = res[2]
	}

	version, err = getVersionFromChannelAndGroup(&ReleasesStatus, vars["channel"], vars["group"])
	if err == nil {
		URLToRedirect = fmt.Sprintf("/%s/documentation/%s/%s", vars["lang"], VersionToURL(version), pageURLRelative)
		err = validateURL(fmt.Sprintf("https://%s%s", r.Host, URLToRedirect))
	}

	if err != nil {
		log.Errorf("Error validating URL: %v, (original was https://%s/%s)", err.Error(), r.Host, r.URL.RequestURI())
		notFoundHandler(w, r)
	} else {
		http.Redirect(w, r, URLToRedirect, 302)
	}
}

// Healthcheck handler
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

// Get HTML content for /includes/topnav.html request
func topnavHandler(w http.ResponseWriter, r *http.Request) {
	_ = updateReleasesStatus()

	versionMenu := versionMenuType{
		VersionItems:           []versionMenuItems{},
		HTMLContent:            "", // not used now
		CurrentGroup:           "", // not used now
		CurrentChannel:         "",
		CurrentLang:            "",
		CurrentVersion:         "",
		CurrentVersionURL:      "",
		CurrentPageURLRelative: "",
		MenuDocumentationLink:  "",
		AbsoluteVersion:        "",
	}

	_ = versionMenu.getVersionMenuData(r)

	tplPath := getRootFilesPath() + r.URL.Path
	tpl := template.Must(template.ParseFiles(tplPath))
	err := tpl.Execute(w, versionMenu)
	if err != nil {
		// Log error or maybe make some magic?
		log.Errorf("Internal Server Error (template error), %s ", err.Error())
		http.Error(w, "Internal Server Error (template error)", 500)
	}
}

func groupMenuHandler(w http.ResponseWriter, r *http.Request) {
	_ = updateReleasesStatus()

	versionMenu := versionMenuType{
		VersionItems:           []versionMenuItems{},
		HTMLContent:            "", // not used now
		CurrentGroup:           "", // not used now
		CurrentChannel:         "",
		CurrentLang:            "",
		CurrentVersion:         "",
		CurrentVersionURL:      "",
		CurrentPageURLRelative: "",
		MenuDocumentationLink:  "",
	}

	_ = versionMenu.getGroupMenuData(r)

	tplPath := getRootFilesPath() + r.RequestURI
	tpl := template.Must(template.ParseFiles(tplPath))
	err := tpl.Execute(w, versionMenu)
	if err != nil {
		// Log error or maybe make some magic?
		log.Errorf("Internal Server Error (template error), %v ", err.Error())
		http.Error(w, "Internal Server Error (template error)", 500)
	}
}

func channelMenuHandler(w http.ResponseWriter, r *http.Request) {
	_ = updateReleasesStatus()

	versionMenu := versionMenuType{
		VersionItems:           []versionMenuItems{},
		HTMLContent:            "", // not used now
		CurrentGroup:           "", // not used now
		CurrentChannel:         "",
		CurrentVersion:         "",
		CurrentLang:            "",
		CurrentVersionURL:      "",
		CurrentPageURLRelative: "",
		MenuDocumentationLink:  "",
	}

	_ = versionMenu.getChannelMenuData(r, &ReleasesStatus)

	tplPath := getRootFilesPath() + r.RequestURI
	tpl := template.Must(template.ParseFiles(tplPath))
	err := tpl.Execute(w, versionMenu)
	if err != nil {
		// Log error or maybe make some magic?
		log.Errorf("Internal Server Error (template error), %v ", err.Error())
		http.Error(w, "Internal Server Error (template error)", 500)
	}
}

func serveFilesHandler(fs http.FileSystem) http.Handler {
	fsh := http.FileServer(fs)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upath := r.URL.Path
		if !strings.HasPrefix(upath, "/") {
			upath = "/" + upath
			r.URL.Path = upath
		}
		upath = path.Clean(upath)
		if _, err := os.Stat(fmt.Sprintf("%v%s", fs, upath)); err != nil {
			if os.IsNotExist(err) {
				notFoundHandler(w, r)
				return
			}
		}
		fsh.ServeHTTP(w, r)
	})
}

func rootDocHandler(w http.ResponseWriter, r *http.Request) {
	var redirectTo, activeRelease string

	log.Debugln("Use handler - rootDocHandler")

	activeRelease = getRootRelease()

	vars := mux.Vars(r)

	if hasSuffix, _ := regexp.MatchString("^/[^/]+/documentation/.+", r.RequestURI); hasSuffix {
		items := strings.Split(r.RequestURI, "/documentation/")
		if len(items) > 1 {
			redirectTo = strings.Join(items[1:], fmt.Sprintf("/%s/documentation/", vars["lang"]))
		}
	}

	http.Redirect(w, r, fmt.Sprintf("/%s/documentation/%s/%s", vars["lang"], activeRelease, redirectTo), 301)
}

// Redirect to root documentation if request not matches any location (override 404 response)
func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusNotFound)
	page404File, err := os.Open(getRootFilesPath() + "/404.html")
	defer page404File.Close()
	if err != nil {
		// 404.html file not found!
		log.Error("404.html file not found")
		http.Error(w, `<html lang="en"><head><meta charset="utf-8">
<meta http-equiv="X-UA-Compatible" content="IE=edge"><title>Page Not Found | werf</title><meta name="title" content="Page Not Found | werf">
</head>
<body>
		<h1 class="docs__title">Page Not Found</h1>
		<p>Sorry, the page you were looking for does not exist.<br>
Try searching for it or check the URL to see if it looks correct.</p>
<div class="error-image">
    <img src="https://werf.io/images/404.png" alt="">
</div>
</body></html>`, 404)
		return
	}
	io.Copy(w, page404File)
}
