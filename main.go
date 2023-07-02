package main

import (
	"context"
	"log"
	"os"

	"github.com/kawaken/eclair/handler"
	"github.com/kawaken/eclair/notify"
)

func main() {
	src := os.Getenv("SRC_DIR")
	if src == "" {
		log.Fatal("SRC_DIR is required")
	}
	dst := os.Getenv("DST_DIR")
	if dst == "" {
		log.Fatal("DST_DIR is required")
	}

	ctx, cancel := context.WithCancel(context.Background())

	errCh := make(chan error)

	converter := handler.NewHLSGenerator(src, dst)
	converter.Start(ctx)

	notifier, err := notify.NewNotifier(src, converter)
	if err != nil {
		cancel()
		log.Fatal(err)
	}

	err = notifier.Scan()
	if err != nil {
		cancel()
		log.Fatal(err)
	}

	err = notifier.Watch(ctx, errCh)
	if err != nil {
		cancel()
		log.Fatal(err)
	}

	for {
		select {
		case err, ok := <-errCh:
			if !ok {
				return
			}
			if err != nil {
				cancel()
				log.Fatal(err)
			}
		}
	}
}
