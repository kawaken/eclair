package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/fsnotify/fsnotify"
)

func scan(srcdir string, target chan string, fileTypeFunc func(string) bool) error {
	files, err := ioutil.ReadDir(srcdir)
	if err != nil {
		return err
	}

	if len(files) > cap(target) {
		return fmt.Errorf(
			"the number of files is greater than the expected number. exp: %d, got: %d",
			cap(target), len(files),
		)
	}

	for _, f := range files {
		if f.IsDir() {
			continue
		}
		if !fileTypeFunc(f.Name()) {
			continue
		}

		target <- filepath.Join(srcdir, f.Name())
	}
	return nil
}

func newWatcher(
	ctx context.Context,
	dir string,
	errCh chan error,
	eventHandler func(fsnotify.Event),
) error {
	// Create new watcher.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	// Add a path.
	err = watcher.Add(dir)
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
				eventHandler(event)

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
