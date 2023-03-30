package main

import (
	"log"
	"os"
	"text/template"

	"golang.org/x/sys/unix"
)

func main() {
	controllerName := os.Getenv("CONTROLLER_NAME")
	if len(controllerName) == 0 {
		log.Fatal("CONTROLLER_NAME env is empty")
	}

	nginxConfTemplate, err := os.ReadFile("/etc/nginx/nginx.conf.tpl")
	if err != nil {
		log.Fatal(err)
	}

	t := template.Must(template.New("nginxTemplate").Parse(string(nginxConfTemplate)))

	fd, err := os.Create("/etc/nginx/nginx.conf")
	if err != nil {
		log.Fatal(err)
	}

	err = t.Execute(fd, map[string]string{"controllerName": controllerName})
	if err != nil {
		log.Fatal(err)
	}

	err = fd.Close()
	if err != nil {
		log.Fatalf("Failed to close file: %s", err)
	}

	err = unix.Exec(os.Args[0], os.Args[1:], os.Environ())
	if err != nil {
		log.Fatal(err)
	}
}
