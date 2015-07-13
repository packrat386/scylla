package main

import (
	"flag"
	"go/build"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/facebookgo/grace/gracehttp"

	"golang.org/x/tools/godoc"
	"golang.org/x/tools/godoc/static"
	"golang.org/x/tools/godoc/vfs"
	"golang.org/x/tools/godoc/vfs/gatefs"
	"golang.org/x/tools/godoc/vfs/mapfs"
)

var (
	pidFile    string
	listenPort string
	logFile    string
	goroot     = flag.String("goroot", runtime.GOROOT(), "Go root directory")
)

func init() {
	flag.StringVar(&listenPort, "port", "3000", "TCP port to run the service on")
	flag.StringVar(&pidFile, "pid", "scylla.pid", "Location to store the PID")
	flag.StringVar(&logFile, "log", "", "Location to store the logs")

	log.SetPrefix("[scylla] ")
}

func main() {
	flag.Parse()

	pid := []byte(strconv.Itoa(os.Getpid()))
	err := ioutil.WriteFile(pidFile, pid, 0644)
	if err != nil {
		log.Fatalf("Could not write to pidfile with error: %s", err.Error())
	}

	if logFile != "" {
		logBuf, err := os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Could not open log file with error: %s", err.Error())
		}
		defer logBuf.Close()
		log.SetOutput(logBuf)
	}

	serve()
}

func indexDirectoryDefault(dir string) bool {
	return dir != "/pkg" && !strings.HasPrefix(dir, "/pkg/")
}

func serve() {
	// use the underlying FS
	fsGate := make(chan bool, 20)
	rootfs := gatefs.New(vfs.OS(*goroot), fsGate)
	fs.Bind("/", rootfs, "/", vfs.BindReplace)

	// bind godoc static files
	fs.Bind("/lib/godoc", mapfs.New(static.Files), "/", vfs.BindReplace)

	// Bind $GOPATH trees into Go root.
	for _, p := range filepath.SplitList(build.Default.GOPATH) {
		fs.Bind("/src", gatefs.New(vfs.OS(p), fsGate), "/src", vfs.BindAfter)
	}

	// make a Corpus
	corpus := godoc.NewCorpus(fs)
	corpus.IndexDirectory = indexDirectoryDefault
	err := corpus.Init()
	if err != nil {
		log.Fatalf("Failed to initialize a corpus: %s", err.Error())
	}

	// make a Presentation
	pres = godoc.NewPresentation(corpus)

	// add handlers to the DefaultServeMux
	readTemplates(pres, true)
	registerHandlers(pres)

	handler := http.DefaultServeMux
	go corpus.RunIndexer()

	handler.HandleFunc("/scylla", scyllaHandler)
	err = gracehttp.Serve(&http.Server{Handler: handler})
	if err != nil {
		log.Fatalf("Couldn't serve http: %s", err.Error())
	}
}
