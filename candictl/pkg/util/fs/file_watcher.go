package fs

import (
	"github.com/fsnotify/fsnotify"

	"github.com/deckhouse/deckhouse/candictl/pkg/log"
)

func StartFileWatcher(path string, fsEventHanlder func(event fsnotify.Event), done chan struct{}) (watcher *fsnotify.Watcher, err error) {
	watcher, err = fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	err = watcher.Add(path)
	if err != nil {
		return nil, err
	}

	log.InfoF("Start watcher for file %s\n", path)

	go func() {
		defer close(done)
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					// watcher.Close() was called
					return
				}
				if fsEventHanlder != nil {
					fsEventHanlder(event)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					// r.stateWatcher.Close() was called
					return
				}
				log.WarnF("fs watcher: %v\n", err.Error())
			}
		}
	}()

	return watcher, nil
}
