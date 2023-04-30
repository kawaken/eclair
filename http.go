package main

import "net/http"

func serveHttp(public string) {
	fs := http.FileServer(http.Dir(public))
	http.Handle("/", fs)
	http.ListenAndServe(":1880", nil)
}
