/*
Copyright 2023 Flant JSC

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

package main

import (
	"flag"
	"log"
	"os"
	"os/exec"
	"strings"
)

var gostsumsPath *string

const resultXMLPath = "/tmp/astra-int-check-report.xml"

func main() {
	if os.Geteuid() != 0 {
		log.Fatalln("This program should be run with root priviliges")
	}

	setupFlags()
	ensureAstraIntCheckInstalled()
	ensureGostsums()

	if err := runChecks(); err != nil {
		log.Fatalln("astra-int-check:", err)
	}

	results, err := parseResult()
	if err != nil {
		log.Fatalln("parsing results:", err)
	}
	defer func() {
		_ = os.Remove(resultXMLPath)
	}()

	if results.Failed != 0 || results.NotFound != 0 {
		log.Printf("TEST FAILED\n\n%v", results)
		os.Exit(1)
	}

	log.Println("TEST PASSED")
	os.Exit(0)
}

func setupFlags() {
	gostsumsPath = flag.String("g", "/tmp/gostsums.txt", "Path to file with gost checksums")
	flag.Parse()
}

func runChecks() error {
	forcedFilter := []string{
		`/boot/*`,
		`/lib/*`,
		`/usr/bin/*`,
		`/lib/security/*`,
		`/etc/init.d/*`,
	}
	ignoreFilter := []string{
		`/etc/*`,
		`/var/*`,
		`/tmp/*`,
		`/proc/*`,
		`/sys/*`,
		`/usr/share/pam-configs/*`,
		`/usr/lib/libreoffice/share/registry/main.xcd`,
		`/usr/lib/libreoffice/share/config/soffice.cfg/modules/*`,
		`/bin/setupcon`,
		`/usr/lib/firefox/browser/defaults/preferences/vendor-firefox.js`,
		`/usr/share/icons/hicolor/index.theme`,
		`/usr/share/icons/hicolor/*/apps/gimp.png`,
		`/usr/sbin/plymouth-set-default-theme`,
		`/usr/sbin/plymouth/debian-logo.png`,
		`/usr/share/vim/vim81/doc/help.txt`,
		`/usr/share/vim/vim81/doc/tags`,
		`/usr/lib/mime/packages/vlc`,
		`/lib/udev/rules.d/91-group-floppy.rules`,
		`/usr/share/knotifications5/astra-event-watcher.notifyrc`,
	}

	checkCmd := exec.Command(
		"/usr/bin/astra-int-check",
		"--gost", *gostsumsPath,
		"--xml", resultXMLPath,
		"--force-filters", strings.Join(forcedFilter, ","),
		"--ignore-filters", strings.Join(ignoreFilter, ","),
	)

	if err := checkCmd.Run(); err != nil {
		return err
	}

	return nil
}
