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

type HLSGenerator struct {
	SrcDir string
	DstDir string
	target chan string
	events *Events
}

func NewHLSGenerator(src, dst string) *HLSGenerator {

	return &HLSGenerator{
		SrcDir: src,
		DstDir: dst,
		target: make(chan string, 100),
		events: NewEvents(),
	}
}

func (h *HLSGenerator) HandleEvent(event fsnotify.Event) {
	switch {
	case event.Has(fsnotify.Create):
		fallthrough
	case event.Has(fsnotify.Write):
		if h.CheckConcerned(event.Name) {
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
	return concernedFileType(name, ".mp4")
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
					if h.CheckConcerned(t.Name) {
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
				err := h.convert(t)
				if err != nil {
					log.Println(err)
				}
			}
		}
	}()

	log.Println("HLSGenerator start")
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
	m3u8Path := filepath.Join(dirPath, "video.m3u8")
	tsBasePath := filepath.Join(dirPath, "video%3d.ts")
	thumbPath := filepath.Join(dirPath, "thumb.jpg")
	htmlPath := filepath.Join(dirPath, "index.html")

	// すでに存在していたらスキップする
	if _, err := os.Stat(m3u8Path); err == nil {
		log.Printf("SKIP exists: %s", m3u8Path)
		return nil
	}

	err := os.MkdirAll(filepath.Join(dirPath), os.ModePerm)
	if err != nil {
		log.Printf("ERROR: cant make dir %q; %s", dirPath, err)
		return err
	}

	// ffmpegを利用して処理を実行する
	log.Printf("Conversion start %s to %s", movFilePath, m3u8Path)
	toHLS := exec.Command("ffmpeg",
		"-i", movFilePath,
		"-c:v", "copy", // ビデオフォーマットはそのまま
		"-c:a", "copy", // オーディオフォーマットはそのまま
		"-f", "hls", // HLSフォーマットとして処理する
		"-hls_time", "6", // 6秒ごとに分割
		"-hls_list_size", "0", // リストのサイズを無制限にする
		"-hls_playlist_type", "vod",
		"-hls_segment_filename", tsBasePath, // video000.ts というファイル名にする
		"-hls_segment_type", "fmp4", // FMP4 フォーマットを利用する
		m3u8Path,
	)
	err = toHLS.Run()
	if err != nil {
		log.Printf("ERROR: ffmpeg %s", err)
		os.RemoveAll(dirPath)
		return err
	} else {
		log.Printf("Conversion complete %s to %s", movFilePath, m3u8Path)
	}

	log.Printf("Conversion start %s to %s", movFilePath, thumbPath)
	toTHM := exec.Command("ffmpeg",
		"-i", movFilePath,
		"-vf", "thumbnail=3600,scale=1280:720", // 開始1分を対象（60fps * 60sec）
		"-frames:v", "1", // 画像は1フレーム
		thumbPath,
	)
	err = toTHM.Run()
	if err != nil {
		log.Printf("ERROR: ffmpeg %s", err)
		os.RemoveAll(dirPath)
		return err
	} else {
		log.Printf("Conversion complete %s to %s", movFilePath, thumbPath)
	}

	// HTMLファイルを生成する
	htmlGen := &HTMLGenerator{
		Title:     movFilename,
		Thumbnail: "thumb.jpg",
		Src:       "video.m3u8",
	}
	err = htmlGen.generateSingle(htmlPath)
	if err != nil {
		log.Printf("ERROR: HTML generation %s", err)
		os.RemoveAll(dirPath)
		return err
	} else {
		log.Printf("Conversion complete %s to %s", movFilePath, htmlPath)
	}

	err = generateIndex(h.DstDir)
	if err != nil {
		log.Printf("ERROR: HTML generation %s", err)
		os.RemoveAll(dirPath)
		return err
	} else {
		log.Printf("Conversion complete %s to %s", movFilePath, filepath.Join(h.DstDir, "index.html"))
	}
	return err
}
