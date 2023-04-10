package main

import (
	"log"
	"os"

	"golang.org/x/sys/unix"
)

func main() {
	controllerName := os.Getenv("CONTROLLER_NAME")
	if len(controllerName) == 0 {
		log.Fatal("CONTROLLER_NAME env is empty")
	}

	nginxConfTemplateBytes, err := os.ReadFile("/etc/nginx/nginx.conf.tpl")
	if err != nil {
		log.Fatal(err)
	}

	nginxConfTemplate := os.ExpandEnv(string(nginxConfTemplateBytes))

	err = os.WriteFile("/etc/nginx/nginx.conf", []byte(nginxConfTemplate), 0666)
	if err != nil {
		log.Fatal(err)
	}

	err = unix.Exec("/usr/sbin/nginx", []string{"nginx", "-g", "daemon off;"}, os.Environ())
	if err != nil {
		log.Fatal(err)
	}
}
