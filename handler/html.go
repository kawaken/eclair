package handler

import (
	_ "embed"
	"html/template"
	"os"
)

//go:embed template.html
var htmlTemplate string
var tpl = template.Must(template.New("main").Parse(htmlTemplate))

type HTMLGenerator struct {
	Title     string
	Thumbnail string
	Src       string
}

func (h *HTMLGenerator) generate(path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return tpl.Execute(f, h)
}
