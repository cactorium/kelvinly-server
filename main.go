package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"syscall"

	"github.com/sevlyar/go-daemon"
	gfm "github.com/shurcooL/github_flavored_markdown"
	"github.com/shurcooL/github_flavored_markdown/gfmstyle"
	//blackfriday "gopkg.in/russross/blackfriday.v2"
)

var (
	signal = flag.String("s", "", `send signal to the daemon
		quit — graceful shutdown
		stop — fast shutdown
		reload — reloading the configuration file`)
)

const DOMAIN_NAME = "threefortiethofonehamster.com"

const HTML_HEADER = `<!doctype html5>
<html>
<head>
  <meta charset=utf-8>
	<title>%s | %s</title>
	<link href=/gfm/gfm.css media=all rel=stylesheet type=text/css></link>
	<link href=/main.css media=all rel=stylesheet type=text/css></link>
</head>
<body>
<nav>
<div class="nav-wrapper">
	<div class="nav-item"><a href="/">Home</a></div>
	<div class="nav-item"><a href="/projects.md">Projects</a></div>
	<div class="nav-item"><a href="/builds.md">Builds</a></div>
	<div class="nav-item"><a href="/resume/resume-KelvinLy-hardware.pdf">Resume</a></div>
</div>
</nav>
<article class="markdown-body entry-content" style="padding:2em;">
`

const HTML_FOOTER = `  </article>
<footer>
<div class="footer-wrapper">
by Kelvin Ly, source available <a href="https://github.com/cactorium/kelvinly-server">here</a>
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
	for _, b := range bs {
		html := gfm.Markdown(b)
		w.Write(html)
	}
	w.Write([]byte(HTML_FOOTER))
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		if r.URL.Path == "/" {
			serveMarkdown(w, r, "static/intro.md", "static/projects.md", "static/builds.md")
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
	shutdown       chan struct{} = make(chan struct{})
	serverShutdown chan struct{} = make(chan struct{})
)

func main() {
	flag.Parse()
	daemon.AddCommand(daemon.StringFlag(signal, "quit"), syscall.SIGQUIT, termHandler)
	daemon.AddCommand(daemon.StringFlag(signal, "stop"), syscall.SIGTERM, termHandler)
	daemon.AddCommand(daemon.StringFlag(signal, "reload"), syscall.SIGHUP, reloadHandler)

	cntxt := &daemon.Context{
		PidFileName: "/tmp/kelvinly-server-pid",
		PidFilePerm: 0644,
		LogFileName: "/tmp/kelvinly-server-log",
		LogFilePerm: 0640,
		//WorkDir:     "/home/kelvin/kelvinly-server/",
		WorkDir: ".",
		Umask:   027,
	}
	// TODO: figure out the daemonizing stuff

	if len(daemon.ActiveFlags()) > 0 {
		d, err := cntxt.Search()
		if err != nil {
			log.Fatalln("Unable to send signal to daemon:", err)
		}
		daemon.SendCommands(d)
		return
	}

	d, err := cntxt.Reborn()
	if err != nil {
		log.Fatalln(err)
	}
	if d != nil {
		return
	}
	defer cntxt.Release()

	var redirect http.Server
	var srv http.Server

	go startRedirectServer(&redirect)
	go startServer(&srv)

	go func() {
		<-shutdown
		log.Println("shutting down server...")
		if err := srv.Shutdown(context.Background()); err != nil {
			log.Printf("server shutdown error: %v\n", err)
		}
		if err = redirect.Shutdown(context.Background()); err != nil {
			log.Printf("redirect shutdown error: %v\n", err)
		}
	}()

	err = daemon.ServeSignals()
	if err != nil {
		log.Println("Error: ", err)
	}

	log.Println("server terminated")
}

func termHandler(sig os.Signal) error {
	log.Printf("sending shutdown signal...")
	close(shutdown)
	return daemon.ErrStop
}

func reloadHandler(sig os.Signal) error {
	log.Printf("[WARN] reloading not supported yet")
	return nil
}

func startServer(srv *http.Server) {
	log.Print("installing handlers")
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", rootHandler)
	//serveMux.Handle("/certbot/", http.StripPrefix("/certbot/", http.FileServer(http.Dir("./certbot-tmp"))))
	serveMux.Handle("/gfm/", http.StripPrefix("/gfm", http.FileServer(gfmstyle.Assets)))
	serveMux.Handle("/resume/", http.StripPrefix("/resume", http.FileServer(http.Dir("resume/"))))
	serveMux.HandleFunc("/main.css", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "main.css") })

	srv.Addr = ":8443"
	srv.Handler = serveMux
	log.Print("starting server")
	/*
		log.Fatal(srv.ListenAndServeTLS("/etc/letsencrypt/live/"+DOMAIN_NAME+"/fullchain.pem",
			"/etc/letsencrypt/live/"+DOMAIN_NAME+"/privkey.pem"))
	*/
	log.Fatal(srv.ListenAndServe())
	close(serverShutdown)
}

func startRedirectServer(srv *http.Server) {
	serveMux := http.NewServeMux()
	// copied from https://gist.github.com/d-schmidt/587ceec34ce1334a5e60
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
