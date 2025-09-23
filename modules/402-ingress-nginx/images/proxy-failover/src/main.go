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
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/ncabatoff/process-exporter/proc"
)

const (
	confTpl              = "/opt/nginx-static/conf/nginx.conf.tpl"
	confNginx            = "/opt/nginx-static/writable/nginx.conf"
	binNginx             = "/opt/nginx-static/sbin/nginx"
	additionalConfigPath = "/opt/nginx-static/additional-conf/accept-requests-from.conf"
	listenAddr           = "127.0.0.1:10255"
	nginxListenAddr      = "127.0.0.1:10253"
)

func prepareConfig() error {
	controllerName := os.Getenv("CONTROLLER_NAME")
	if len(controllerName) == 0 {
		return fmt.Errorf("init error: CONTROLLER_NAME env is empty")
	}

	nginxConfTemplateBytes, err := os.ReadFile(confTpl)
	if err != nil {
		return fmt.Errorf("reading config template failed: %v", err)
	}

	nginxConfTemplate := os.ExpandEnv(string(nginxConfTemplateBytes))

	err = os.WriteFile(confNginx, []byte(nginxConfTemplate), 0666)
	if err != nil {
		return fmt.Errorf("writing config failed: %v", err)
	}
	return nil
}

func startNginx() (int, error) {
	cmd := exec.Command(binNginx, "-c", confNginx, "-e", "stderr")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// put NGINX in another process group to prevent it
	// to receive signals meant for the controller
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
		Pgid:    0,
	}
	err := cmd.Start()
	if err != nil {
		return 0, err
	}

	go func() {
		err := cmd.Wait()
		if err != nil {
			log.Printf("harvesting child's return code: %v", err)
		}
	}()

	return cmd.Process.Pid, nil
}

func isNginxMasterRunning(pid int) error {
	// check the nginx master process is running
	fs, err := proc.NewFS("/proc", false)
	if err != nil {
		return fmt.Errorf("could not read /proc directory: %w", err)
	}

	_, err = fs.Proc(pid)
	if err != nil {
		return fmt.Errorf("could not check for NGINX process with PID %v: %w", pid, err)
	}

	return nil
}

func stopNginx() (string, error) {
	output, err := exec.Command(binNginx, "-s", "quit", "-e", "/dev/null").CombinedOutput()
	return string(output), err
}

func testConfig() (string, error) {
	output, err := exec.Command(binNginx, "-t", "-c", confNginx, "-e", "/dev/null").CombinedOutput()
	return string(output), err
}

func reloadConfig() (string, error) {
	output, err := exec.Command(binNginx, "-s", "reload", "-c", confNginx, "-e", "/dev/null").CombinedOutput()
	return string(output), err
}

func checker(w http.ResponseWriter, pid int) {
	if err := isNginxMasterRunning(pid); err != nil {
		log.Printf("could not find nginx master process: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("500 - nginx master process not found"))
		return
	}

	res, err := http.Get(fmt.Sprintf("http://%s/healthz", nginxListenAddr))
	if err != nil {
		log.Printf("could not request nginx /healthz: %v", err)
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("503 - nginx server unavailable"))
		return
	}

	if res.StatusCode != http.StatusOK {
		log.Printf("could not get 200 response code from nginx: %v", err)
		w.WriteHeader(res.StatusCode)
		w.Write([]byte("fail"))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ping"))
}

func main() {

	err := prepareConfig()
	if err != nil {
		log.Fatalf("could not prepare nginx config: %v", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("could not create watch: %v", err)
	}
	defer watcher.Close()

	err = watcher.Add(additionalConfigPath)
	if err != nil {
		log.Fatalf("could not add file to watcher: %v", err)
	}

	pid, err := startNginx()
	if err != nil {
		log.Fatalf("could not start nginx process: %v", err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
			checker(w, pid)
		})

		if err := http.ListenAndServe(listenAddr, nil); err != nil && err != http.ErrServerClosed {
			log.Fatalf("could not listen on %s: %v", listenAddr, err)
		}
	}()

	log.Printf("started nginx daemon with PID %d, starting control loop", pid)
loop:
	for {
		select {

		case event := <-watcher.Events:
			if event.Op == fsnotify.Remove {
				_ = watcher.Remove(event.Name)
				if err := watcher.Add(event.Name); err != nil {
					log.Fatalf("could not add file to watcher: %v", err)
				}

				switch event.Name {
				case additionalConfigPath:
					log.Println("nginx config has been updated and will be reloaded")
					if output, err := testConfig(); err != nil {
						log.Printf("nginx test config failed: %s", output)
					} else {
						output, err := reloadConfig()
						if err != nil {
							log.Printf("could not reload nginx config: %v", err)
						}
						if len(output) > 0 {
							log.Print(output)
						}
					}
				}
			}

		case err := <-watcher.Errors:
			log.Printf("watch files error: %s\n", err)

		case sig := <-sigs:
			log.Printf("caught %s signal, terminating...", sig)
			break loop
		}
	}

	output, err := stopNginx()
	if err != nil {
		log.Fatalf("stopping nginx: %v", err)
	}

	if len(output) > 0 {
		log.Printf(output)
	}

	for {
		if err := isNginxMasterRunning(pid); err != nil {
			return
		}
		time.Sleep(time.Second * 1)
	}
}
