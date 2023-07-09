package handler

import (
	_ "embed"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

//go:embed single.html
var singleTemplate string

//go:embed index.html
var indexTemplate string

var (
	single = template.Must(template.New("main").Parse(singleTemplate))
	index  = template.Must(template.New("main").Parse(indexTemplate))
)

type HTMLGenerator struct {
	Title     string
	Thumbnail string
	Src       string
}

func (h *HTMLGenerator) generateSingle(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return single.Execute(f, h)
}

type Page struct {
	Title     string
	Thumbnail string
	Path      string
}

func generateIndex(rootDirPath string) error {
	pages := make([]Page, 0)
	maxDepth := 1 + strings.Count(rootDirPath, string(os.PathSeparator))

	err := filepath.WalkDir(rootDirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if !d.IsDir() {
			return nil
		}

		if path == rootDirPath {
			return nil
		}

		if strings.Count(path, string(os.PathSeparator)) > maxDepth {
			log.Println("skip", path)
			return fs.SkipDir
		}

		// check thumb.jpg
		name := d.Name()
		thumbnail := filepath.Join(rootDirPath, name, "thumb.jpg")
		if _, e := os.Stat(thumbnail); e != nil {
			log.Println(e)
			return fs.SkipDir
		}

		pages = append(pages, Page{
			Title:     name,
			Thumbnail: thumbnail,
			Path:      fmt.Sprintf("./%s/", name),
		})

		log.Printf("Add page to index: %s", name)
		return nil
	})
	if err != nil {
		return err
	}

	f, err := os.Create(filepath.Join(rootDirPath, "index.html"))
	if err != nil {
		return err
	}
	defer f.Close()

	return index.Execute(f, pages)

}
