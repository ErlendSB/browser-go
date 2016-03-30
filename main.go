package main

import (
	"flag"
	"io/ioutil"
	"log"
	"fmt"
	"net/http"
	"time"
)

var port *int = flag.Int("port", 4004, "port")
var cacheLength *int = flag.Int("secs", 3*60*60, "cache retention in seconds (3 hours)")
var webkitInstances *int = flag.Int("webkits", 5, "amount of webkits in pool")

var phantom *Phantom = NewWebkitPool(*webkitInstances)

func param(r *http.Request, name string) string {
	if len(r.Form[name]) > 0 {
		return r.Form[name][0]
	}
	return ""
}

func serveFile(w http.ResponseWriter, r *http.Request, filename string) {
	http.ServeFile(w, r, filename)
}

// servePng takes a byte slice and flushes it on the wire
// treating it as an image/png mime type
func (p *Process) ServePng(b []byte) {
	// mime
	p.writer.Header().Set("Content-Type", "image/png")

	// 3 hours
	p.writer.Header().Set("Cache-Control", "public, max-age=10800")
	p.writer.WriteHeader(http.StatusOK)
	p.writer.Write(b)
	p.bytesWritten = len(b)
	p.status = http.StatusOK
}

// Return true if the cache entry should be considered
// fresh based on the command line parameters
func fresh(c *cacheEntry) bool {
	elapsed := time.Since(c.stat.ModTime()).Minutes()
	return elapsed < float64(*cacheLength)*3
}

func (p *Process) ServeError(msg string) {
	log.Println(msg)
	p.status = http.StatusInternalServerError
	http.Error(p.writer, msg, http.StatusInternalServerError)
}

type Process struct {
	writer  http.ResponseWriter
	request *http.Request

	screenshotUrl  string
	bytesWritten     int

	status int
}

func (p *Process) Log() {
	log.Printf("GET status:%d url:%s bytes:%d ", p.status, p.screenshotUrl, p.bytesWritten)
}

func (p *Process) Handle() {
	defer p.Log()
	var buffer []byte

	// make the screenshot
	filename := phantom.Screenshot(p.screenshotUrl)

	if filename == "" {
		p.ServeError("Error creating screenshot")
		return
	}

	png, err := ioutil.ReadFile(filename)
	if err == nil {
		buffer = png
	}

	p.ServePng(buffer)
	return

}

func Server(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	process := Process{writer: w, request: r}

	if len(r.Form["src"]) > 0 {
		process.screenshotUrl = r.Form["src"][0]
	} else {
		http.NotFound(w, r)
		return
	}

	process.Handle()
	return
}

func main() {
	flag.Parse()

	http.HandleFunc("/favicon.ico", http.NotFound)
	http.HandleFunc("/", Server)

	binding := fmt.Sprintf(":%d", *port)
	log.Printf("Running and listening to port %d", *port)

	if err := http.ListenAndServe(binding, nil); err != nil {
		log.Panicln("Could not start server:", err)
	}
}
