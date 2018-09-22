package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"

	gfm "github.com/shurcooL/github_flavored_markdown"
	"github.com/shurcooL/github_flavored_markdown/gfmstyle"
	//blackfriday "gopkg.in/russross/blackfriday.v2"
)

const HTML_HEADER = `<!doctype html5>
<html>
<head>
  <meta charset=utf-8>
	<title>%s | %s</title>
	<link href=/gfm/gfm.css media=all rel=stylesheet type=text/css></link>
	<link href=/main.css media=all rel=stylesheet type=text/css></link>
</head>
<body>
<article class="markdown-body entry-content" style="padding:2em;">
`

const HTML_FOOTER = `  </article>
</body>
</html>`

func serveMarkdown(w http.ResponseWriter, r *http.Request, path string) {
	if b, err := ioutil.ReadFile(path); err != nil {
		w.WriteHeader(404)
		w.Write([]byte(fmt.Sprintf("file %s not found", path)))
		return
	} else {
		w.Header().Add("Content-Type", "text/html; charset=utf-8")
		title := ""
		if s := bytes.Index(b, []byte("# ")); s != -1 {
			t := b[s+2:]
			if e := bytes.Index(t, []byte("\n")); e != -1 {
				t = t[:e]
				title = string(t)
			}
		}
		w.Write([]byte(fmt.Sprintf(HTML_HEADER, string(title), r.Host)))
		html := gfm.Markdown(b)
		w.Write(html)
		w.Write([]byte(HTML_FOOTER))
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if r.URL.Path == "/" {
			serveMarkdown(w, r, "static/README.md")
		} else if strings.HasSuffix(r.URL.Path, ".md") {
			if strings.Contains(r.URL.Path, "..") {
				w.WriteHeader(403)
				w.Write([]byte("\"..\" forbidden in URL"))
				return
			}
			filepath := "static" + r.URL.Path
			serveMarkdown(w, r, filepath)
		} else {
			http.ServeFile(w, r, "static"+r.URL.Path)
		}
	} else {
		w.Write([]byte("unimplemented!"))
	}
}

func main() {
	log.Print("installing handlers")
	http.HandleFunc("/", rootHandler)
	http.Handle("/gfm/", http.StripPrefix("/gfm", http.FileServer(gfmstyle.Assets)))
	http.HandleFunc("/main.css", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "main.css") })
	log.Print("starting server")
	log.Fatal(http.ListenAndServe(":80", nil))
}
