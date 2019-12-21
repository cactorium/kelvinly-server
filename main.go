package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"io/ioutil"

	"github.com/shurcooL/github_flavored_markdown/gfmstyle"
	//blackfriday "gopkg.in/russross/blackfriday.v2"
)

const DEBUG = false

const DOMAIN_NAME = "threefortiethofonehamster.com"

const HTML_HEADER = `<!doctype html5>
<html>
<head>
  <meta charset=utf-8>
	<meta name="viewport" content="width=device-width, initial-scale=1">
	<title>%s | %s</title>
	<link href=/gfm/gfm.css media=all rel=stylesheet type=text/css></link>
	<link href=/main.css media=all rel=stylesheet type=text/css></link>
</head>
<body>
<nav>
<div class="nav-wrapper">
	<div class="nav-item"><a href="/">Home</a></div>
	<div class="nav-item"><a href="/builds.md">Projects</a></div>
	<div class="nav-item"><a href="https://git.threefortiethofonehamster.com/">Code</a></div>
	<div class="nav-item"><a href="/resume/resume-KelvinLy-hardware.pdf">Resume</a></div>
</div>
</nav>
<article class="markdown-body entry-content" style="padding:4em;">
`

const HTML_FOOTER = `
</article>
<footer>
<div class="footer-wrapper">
by Kelvin Ly, source available <a href="https://github.com/cactorium/threefortiethofonehamster.com">here</a>
</div>
</footer>
</body>
</html>`

func serveMarkdown(w http.ResponseWriter, r *http.Request, paths ...string) {
	bs := make([][]byte, 0, len(paths))
	for _, path := range paths {
		if b, err := ioutil.ReadFile(path); err != nil {
			w.WriteHeader(404)
			w.Write([]byte(fmt.Sprintf("file %s not found", path)))
			return
		} else {
			bs = append(bs, b)
		}
	}
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	title := ""
	if s := bytes.Index(bs[0], []byte("# ")); s != -1 {
		t := bs[0][s+2:]
		if e := bytes.Index(t, []byte("\n")); e != -1 {
			t = t[:e]
			title = string(t)
		}
	}
	w.Write([]byte(fmt.Sprintf(HTML_HEADER, string(title), r.Host)))
	for i, b := range bs {
		pathDir := paths[i][len("static/"):]
		lastSlash := strings.LastIndex(pathDir, "/")
		if lastSlash != -1 {
			pathDir = pathDir[:lastSlash]
		}
		// Markdown uses the path to generate the correct paths for resized images
		html := Markdown(b, pathDir)
		w.Write(html)
	}
	w.Write([]byte(HTML_FOOTER))
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if r.URL.Path == "/" {
			serveMarkdown(w, r, "static/intro.md", "static/builds.md")
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

var (
	serverShutdown chan struct{} = make(chan struct{})
)

func main() {
	flag.Parse()

	var redirect http.Server
	var srv http.Server

	go startRedirectServer(&redirect)
	go startServer(&srv)

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt)

	<-shutdown
	log.Println("shutting down server...")
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Printf("server shutdown error: %v\n", err)
	}
	if err := redirect.Shutdown(context.Background()); err != nil {
		log.Printf("redirect shutdown error: %v\n", err)
	}

	log.Println("server terminated")
}

func readWebhookKey() []byte {
	b, err := ioutil.ReadFile("webhook_secret")
	if err != nil {
		log.Printf("[ERR] webhook key not found, webhook updates will not work!")
		return nil
	}
	/*
		ret := make([]byte, hex.DecodedLen(len(b)))
		// skip the ending 0x0a
		rl, err2 := hex.Decode(ret, b[:len(b)-1])
		if err2 != nil {
			log.Printf("[ERR] unable to decode webhook key! %v %s", b, err2)
			return nil
		}
	*/

	return b[:len(b)-1]
}

func startServer(srv *http.Server) {
	log.Print("installing handlers")

	webhookKey := readWebhookKey()

	serveMux := http.NewServeMux()
	url, err := url.Parse("http://localhost:8081")
	if err != nil {
		log.Fatalf("unable to parse reverse proxy path: %v", err)
		return
	}
	serveMux.Handle("dev."+DOMAIN_NAME+"/", httputil.NewSingleHostReverseProxy(url))

	gogsUrl, err := url.Parse("http://localhost:7000")
	if err != nil {
		log.Fatalf("unable to parse reverse proxy path: %v", err)
		return
	}
	serveMux.Handle("git."+DOMAIN_NAME+"/", httputil.NewSingleHostReverseProxy(gogsUrl))

	serveMux.HandleFunc("/", rootHandler)
	//serveMux.Handle("/certbot/", http.StripPrefix("/certbot/", http.FileServer(http.Dir("./certbot-tmp"))))
	serveMux.Handle("/gfm/", http.StripPrefix("/gfm", http.FileServer(gfmstyle.Assets)))
	serveMux.Handle("/resume/", http.StripPrefix("/resume", http.FileServer(http.Dir("resume/"))))
	serveMux.Handle("/resize/", Cache(Resize(640, http.StripPrefix("/resize", http.FileServer(http.Dir("static/"))))))
	serveMux.HandleFunc("/main.css", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "main.css") })
	if webhookKey != nil {
		log.Print("web hook found")
		serveMux.HandleFunc("/update", func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				w.WriteHeader(403)
				w.Write([]byte("invalid request type"))
				return
			}
			signature := r.Header.Get("X-Hub-Signature")
			if len(signature) == 0 {
				w.WriteHeader(403)
				w.Write([]byte("invalid request"))
				return
			}

			payload, e := ioutil.ReadAll(r.Body)
			if e != nil {
				w.WriteHeader(403)
				w.Write([]byte("unable to read body: " + e.Error()))
				return
			}

			mac := hmac.New(sha1.New, webhookKey)
			mac.Write(payload)
			expected := mac.Sum(nil)

			signatureDec := make([]byte, hex.DecodedLen(len(signature)))
			// skip the "sha1=" part
			sdl, e2 := hex.Decode(signatureDec, []byte(signature)[5:])
			if e2 != nil {
				w.WriteHeader(403)
				w.Write([]byte("unable to read signature"))
				return
			}

			signatureDec = signatureDec[:sdl]
			if !hmac.Equal(expected, signatureDec) {
				log.Print("webhook hmac match failed; expected %v found %v", expected, signatureDec)
				w.WriteHeader(403)
				w.Write([]byte("invalid request"))
				return
			}
			// TODO parse payload

			pullCmd := exec.Command("git", "pull")
			pullCmd.Dir = "./static/"
			_ = pullCmd.Run()

			w.Write([]byte("success"))
		})
	}

	srv.Addr = ":8443"
	srv.Handler = Gzip(serveMux)
	log.Print("starting server at " + srv.Addr)
	if !DEBUG {
		log.Fatal(srv.ListenAndServeTLS("/etc/letsencrypt/live/"+DOMAIN_NAME+"/fullchain.pem",
			"/etc/letsencrypt/live/"+DOMAIN_NAME+"/privkey.pem"))
	} else {
		log.Fatal(srv.ListenAndServe())
	}
	close(serverShutdown)
}

func startRedirectServer(srv *http.Server) {
	serveMux := http.NewServeMux()
	// copied from https://gist.github.com/d-schmidt/587ceec34ce1334a5e60
	url, err := url.Parse("http://localhost:8081")
	if err != nil {
		log.Fatalf("unable to parse reverse proxy path: %v", err)
		return
	}
	serveMux.Handle("dev."+DOMAIN_NAME+"/", httputil.NewSingleHostReverseProxy(url))

	serveMux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		target := "https://" + req.Host + req.URL.Path
		if len(req.URL.RawQuery) > 0 {
			target += "?" + req.URL.RawQuery
		}
		http.Redirect(w, req, target, http.StatusTemporaryRedirect)
	})

	srv.Addr = ":8080"
	srv.Handler = serveMux
	log.Print("starting server")
	log.Fatal(srv.ListenAndServe())
	close(serverShutdown)
}
