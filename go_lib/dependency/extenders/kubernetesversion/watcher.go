package kubernetesversion

import (
	"os"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
	"github.com/fsnotify/fsnotify"
)

type versionWatcher struct {
	ch          chan<- *semver.Version
	lastVersion *semver.Version
	watcher     *fsnotify.Watcher
}

func (w *versionWatcher) watch(path string) (err error) {
	if err = waitForExisting(path); err != nil {
		return err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(data)) != "" {
		parsed, err := semver.NewVersion(strings.TrimSpace(string(data)))
		if err != nil {
			return err
		}
		w.lastVersion = parsed
		w.ch <- parsed
	}

	if w.watcher, err = fsnotify.NewWatcher(); err != nil {
		return err
	}
	if err = w.watcher.Add(path); err != nil {
		return err
	}
	for {
		select {
		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}
			if data, err = os.ReadFile(path); err != nil {
				return err
			}
			if err = w.handler(string(data), event); err != nil {
				return err
			}
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return nil
			}
			return err
		}
	}
}

func (w *versionWatcher) handler(content string, _ fsnotify.Event) error {
	parsed, err := semver.NewVersion(strings.TrimSpace(content))
	if err != nil {
		return err
	}
	if w.lastVersion == nil || !w.lastVersion.Equal(parsed) {
		w.lastVersion = parsed
		w.ch <- w.lastVersion
	}
	return nil
}

func waitForExisting(path string) error {
	for {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if os.IsNotExist(err) {
			time.Sleep(10 * time.Millisecond)
		} else {
			return err
		}
	}
}
