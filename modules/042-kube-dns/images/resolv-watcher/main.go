package main

import (
	"log"
	"os"
	"strconv"

	"github.com/fsnotify/fsnotify"
	"golang.org/x/sys/unix"
)

func main() {
	corednsPID, err := getCoreDnsPID()
	if err != nil {
		log.Fatalf("failed to get CoreDNS PID: %s", err)
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	defer func(watcher *fsnotify.Watcher) {
		err := watcher.Close()
		if err != nil {
			log.Fatalf("Can't close fsnotify watcher: %s", err)
		}
	}(watcher)

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event: ", event)
				if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename) {
					err := unix.Kill(corednsPID, unix.SIGUSR1)
					if err != nil {
						log.Fatalf("failed to send SIGUSR1 to coredns PID %v: %s", corednsPID, err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				// TODO: add health checking machinery
				log.Fatalf("inotify watcher returned error: %s", err)
			}
		}
	}()

	// TODO: watch whole directory and react to resolv.conf files changes
	err = watcher.Add("/etc/resolv.conf")
	if err != nil {
		log.Fatal(err)
	}

	// TODO: add signal handling
	<-make(chan struct{})
}

func getCoreDnsPID() (int, error) {
	pid, err := os.ReadFile("/tmp/coredns.pid")
	if err != nil {
		return 0, err
	}

	return strconv.Atoi(string(pid))
}
