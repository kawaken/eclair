package handler

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

const targetBuffer = 100

type HLSGenerator struct {
	SrcDir string
	DstDir string
	target chan string
	events *Events
}

type Converted struct {
	From string
	To   string
	Err  error
}

func NewHLSGenerator(src, dst string) *HLSGenerator {

	return &HLSGenerator{
		SrcDir: src,
		DstDir: dst,
		target: make(chan string, targetBuffer),
		events: NewEvents(),
	}
}

func (h *HLSGenerator) isValidFileType(name string) bool {
	ext := strings.ToLower(filepath.Ext(name))
	return ext == ".mp4"
}

func (h *HLSGenerator) HandleEvent(event fsnotify.Event) {
	switch {
	case event.Has(fsnotify.Write):
		if h.isValidFileType(event.Name) {
			h.events.Set(event)
		}
	case event.Has(fsnotify.Rename):
		// Renameイベントは古いファイル名に対して発生する
		// 同じディレクトリにある場合は新しいファイルでCREATEが起きるため除去する
		h.events.Remove(event)
	case event.Has(fsnotify.Remove):
		h.events.Remove(event)
	case event.Has(fsnotify.Chmod):
		// ignore event
	default:
		log.Println(event)
	}
}

func (h *HLSGenerator) HandleScannedFiles(files []string) {
	for _, f := range files {
		h.target <- f
	}
}

func (h *HLSGenerator) CheckConcerned(name string) bool {
	return h.isValidFileType(name)
}

func (h *HLSGenerator) Start(ctx context.Context) {
	// 10秒ごとにイベント発生する
	ticker := time.NewTicker(10 * time.Second)

	// fsnotify.Writeが発生しなくなってから時間経過したファイルを変換処理の対象とする
	go func() {
		for {
			select {
			case <-ctx.Done():
				// 終了されたらループを抜ける
				return
			case <-ticker.C:
				targets := h.events.VerifyExpiredEvents()

				for _, t := range targets {
					if h.isValidFileType(t.Name) {
						h.target <- t.Name
					}
				}
			}
		}
	}()

	// 変換処理を非同期で実行する
	go func() {
		for {
			select {
			case <-ctx.Done():
				// 終了されたらループを抜ける
				return
			case t, ok := <-h.target:
				if !ok {
					// targetがcloseされていたら終了
					return
				}
				// convert処理のエラーは無視する
				// ログを見て対応する
				h.convert(t)
			}
		}
	}()
}

func (h *HLSGenerator) convert(movFilePath string) error {
	// 処理対象のファイル
	log.Printf("Target: %s", movFilePath)

	// 絶対パスのみ扱う
	if !filepath.IsAbs(movFilePath) {
		return fmt.Errorf("path is not absolute path: %s", movFilePath)
	}

	// ファイル名を利用して新規ディレクトリを作成する
	movFilename := filepath.Base(movFilePath)
	dirName := strings.TrimSuffix(movFilename, ".mp4")
	dirPath := filepath.Join(h.DstDir, dirName)
	// リストのファイル名は固定
	outputPath := filepath.Join(dirPath, "video.m3u8")
	tsBasePath := filepath.Join(dirPath, "video%3d.ts")

	// すでに存在していたらスキップする
	if _, err := os.Stat(outputPath); err == nil {
		log.Printf("SKIP exists: %s", outputPath)
		return nil
	}

	err := os.MkdirAll(filepath.Join(dirPath), os.ModePerm)
	if err != nil {
		log.Printf("ERROR: cant make dir %q; %s", dirPath, err)
		return err
	}

	// ffmpegを利用して処理を実行する
	log.Printf("Conversion start %s to %s", movFilePath, outputPath)
	cmd := exec.Command("ffmpeg",
		"-i", movFilePath,
		"-c:v", "copy", // ビデオフォーマットはそのまま
		"-c:a", "copy", // オーディオフォーマットはそのまま
		"-f", "hls", // HLSフォーマットとして処理する
		"-hls_time", "6", // 6秒ごとに分割
		"-hls_list_size", "0", // リストのサイズを無制限にする
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", tsBasePath, // video000.ts というファイル名にする
		"-hls_segment_type", "fmp4", // FMP4 フォーマットを利用する
		outputPath,
	)
	//log.Println(cmd.String())
	err = cmd.Run()
	if err != nil {
		log.Printf("ERROR: ffmpeg %s", err)
		os.RemoveAll(dirPath)
	} else {
		log.Printf("Conversion complete %s to %s", movFilePath, outputPath)
	}

	return err
}
