package main

import (
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

func main() {
	backend, err := url.Parse("http://127.0.0.1:8081")
	if err != nil {
		log.Fatal(err)
	}

	proxy := httputil.NewSingleHostReverseProxy(backend)
	static := http.FileServer(http.Dir("resources/nginx-1.18.0/html/hmdp"))

	http.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = strings.TrimPrefix(r.URL.Path, "/api")
		if r.URL.Path == "" {
			r.URL.Path = "/"
		}
		proxy.ServeHTTP(w, r)
	})
	http.Handle("/", static)

	log.Println("frontend listening on http://127.0.0.1:8090")
	log.Fatal(http.ListenAndServe("127.0.0.1:8090", nil))
}
