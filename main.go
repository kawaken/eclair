package main

import (
	"context"
	"log"
	"os"
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

	converter := NewConverter(src, dst)
	converter.InitScan()
	errCh, err := converter.WatchAndConvert(ctx)

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
