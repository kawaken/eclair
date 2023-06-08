package notify

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

type Handler interface {
	HandleEvent(fsnotify.Event)
	HandleScannedFiles([]string)
	CheckConcerned(string) bool
}

type Notifier struct {
	*fsnotify.Watcher
	Handlers []Handler
	Dir      string
}

func NewNotifier(
	dir string,
	handlers ...Handler,
) (*Notifier, error) {
	// Create new watcher.
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if handlers == nil {
		handlers = make([]Handler, 0)
	}

	watcher := &Notifier{
		Watcher:  fsw,
		Handlers: handlers,
		Dir:      dir,
	}

	return watcher, nil
}

func (watcher *Notifier) Scan() error {
	files, err := ioutil.ReadDir(watcher.Dir)
	if err != nil {
		return err
	}

	filepathList := make([]string, 0, len(files))
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		filepathList = append(filepathList, filepath.Join(watcher.Dir, f.Name()))
	}

	for _, h := range watcher.Handlers {
		wants := make([]string, 0)
		for _, f := range filepathList {
			if h.CheckConcerned(f) {
				wants = append(wants, f)
			}
		}
		if len(wants) > 0 {
			go h.HandleScannedFiles(wants)
		}
	}
	return nil
}

func (watcher *Notifier) Watch(
	ctx context.Context,
	errCh chan error,
) error {

	// Add a path.
	err := watcher.Add(watcher.Dir)
	if err != nil {
		return err
	}

	go func() {
		// 非同期処理を終了する際にwatcherをCloseする
		defer func() {
			watcher.Close()
		}()

		for {
			select {
			case <-ctx.Done():
				// 終了されたらループを抜ける
				return
			case event, ok := <-watcher.Events:
				if !ok {
					errCh <- fmt.Errorf("watcher.Events closed")
					return
				}

				for _, h := range watcher.Handlers {
					if h.CheckConcerned(event.Name) {
						go h.HandleEvent(event)
					}
				}

			case err, ok := <-watcher.Errors:
				if !ok {
					errCh <- fmt.Errorf("watcher.Errors closed")
					return
				}
				errCh <- err
				return
			}
		}
	}()

	return nil
}
