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
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"compress/gzip"
	"io"
	"io/ioutil"
	"sync"

	"github.com/sevlyar/go-daemon"
	gfm "github.com/shurcooL/github_flavored_markdown"
	"github.com/shurcooL/github_flavored_markdown/gfmstyle"
	//blackfriday "gopkg.in/russross/blackfriday.v2"
)

// code copied from https://gist.github.com/CJEnright/bc2d8b8dc0c1389a9feeddb110f822d7

var gzPool = sync.Pool{
	New: func() interface{} {
		w := gzip.NewWriter(ioutil.Discard)
		return w
	},
}

type gzipResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *gzipResponseWriter) WriteHeader(status int) {
	w.Header().Del("Content-Length")
	w.ResponseWriter.WriteHeader(status)
}

func (w *gzipResponseWriter) Write(b []byte) (int, error) {
	return w.Writer.Write(b)
}

func Gzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}

		w.Header().Set("Content-Encoding", "gzip")

		gz := gzPool.Get().(*gzip.Writer)
		defer gzPool.Put(gz)

		gz.Reset(w)
		defer gz.Close()

		next.ServeHTTP(&gzipResponseWriter{ResponseWriter: w, Writer: gz}, r)
	})
}

var (
	signal = flag.String("s", "", `send signal to the daemon
		quit — graceful shutdown
		stop — fast shutdown
		reload — reloading the configuration file`)
	devmode = flag.Bool("dev_mode", false, "whether this server should run in developer mode or not")
)

const DEBUG = false

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
		PidFileName: "/tmp/main-server-pid",
		PidFilePerm: 0644,
		LogFileName: "/tmp/main-server-log",
		LogFilePerm: 0640,
		WorkDir:     "/home/kelvin/main-server/",
		Umask:       027,
	}
	if *devmode {
		cntxt = &daemon.Context{
			PidFileName: "/tmp/dev-server-pid",
			PidFilePerm: 0644,
			LogFileName: "/tmp/dev-server-log",
			LogFilePerm: 0640,
			WorkDir:     "/home/kelvin/dev-server/",
			Umask:       027,
		}

	}
	if DEBUG {
		cntxt.WorkDir = "."
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

// copied from https://stackoverflow.com/questions/34724160/go-http-send-incoming-http-request-to-an-other-server-using-client-do
func forwardRequest(port int, proxyScheme string) func(http.ResponseWriter, *http.Request) {
	proxyHost := "0.0.0.0" + ":" + strconv.Itoa(port)
	return func(w http.ResponseWriter, req *http.Request) {
		// we need to buffer the body if we want to read it here and send it
		// in the request.
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// you can reassign the body if you need to parse it as multipart
		req.Body = ioutil.NopCloser(bytes.NewReader(body))

		// create a new url from the raw RequestURI sent by the client
		url := fmt.Sprintf("%s://%s%s", proxyScheme, proxyHost, req.RequestURI)

		proxyReq, err := http.NewRequest(req.Method, url, bytes.NewReader(body))

		// We may want to filter some headers, otherwise we could just use a shallow copy
		// proxyReq.Header = req.Header
		proxyReq.Header = make(http.Header)
		for h, val := range req.Header {
			proxyReq.Header[h] = val
		}

		resp, err := (&http.Client{}).Do(proxyReq)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		defer resp.Body.Close()

		// legacy code
	}
}

func startServer(srv *http.Server) {
	log.Print("installing handlers")

	webhookKey := readWebhookKey()

	serveMux := http.NewServeMux()
	if !*devmode {
		serveMux.HandleFunc("dev."+DOMAIN_NAME+"/", forwardRequest(8444, "https"))
	}
	serveMux.HandleFunc("/", rootHandler)
	//serveMux.Handle("/certbot/", http.StripPrefix("/certbot/", http.FileServer(http.Dir("./certbot-tmp"))))
	serveMux.Handle("/gfm/", http.StripPrefix("/gfm", http.FileServer(gfmstyle.Assets)))
	serveMux.Handle("/resume/", http.StripPrefix("/resume", http.FileServer(http.Dir("resume/"))))
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

	if *devmode {
		srv.Addr = ":8444"
	} else {
		srv.Addr = ":8443"
	}
	srv.Handler = Gzip(serveMux)
	log.Print("starting server")
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
	if !*devmode {
		serveMux.HandleFunc("dev."+DOMAIN_NAME+"/", forwardRequest(8081, "http"))
	}

	serveMux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		target := "https://" + req.Host + req.URL.Path
		if len(req.URL.RawQuery) > 0 {
			target += "?" + req.URL.RawQuery
		}
		http.Redirect(w, req, target, http.StatusTemporaryRedirect)
	})

	if *devmode {
		srv.Addr = ":8081"
	} else {
		srv.Addr = ":8080"
	}
	srv.Handler = serveMux
	log.Print("starting server")
	log.Fatal(srv.ListenAndServe())
	close(serverShutdown)
}
