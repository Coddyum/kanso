// kanso: a small local app to follow business expenses, upcoming
// payments and invoices, kept in sync with a folder of the Mac (~/Documents/Inc).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/coddyum/kanso/internal/brand"
	"github.com/coddyum/kanso/internal/server"
	"github.com/coddyum/kanso/web"
)

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("home folder: %v", err)
	}
	port := flag.Int("port", 48219, "HTTP port (listens on 127.0.0.1 only)")
	root := flag.String("root", filepath.Join(home, "Documents", "Inc"), "the folder the app keeps in step")
	flag.Parse()

	if st, err := os.Stat(*root); err != nil || !st.IsDir() {
		log.Fatalf("folder not found: %s", *root)
	}
	pdftext := findPDFText()
	srv, err := server.New(server.Config{
		Root: *root, CacheDir: filepath.Join(home, "Library", "Caches", brand.ID),
		PDFText: pdftext, Port: *port, Static: web.Files,
	})
	if err != nil {
		log.Fatal(err)
	}
	httpSrv := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", *port),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("%s → http://localhost:%d  (folder: %s)", brand.ID, *port, *root)
	if pdftext == "" {
		log.Printf("pdftext not found: PDFs are listed without their amount (run `make build`)")
	}

	// The launcher stops the app with SIGTERM on an update: requests in flight finish first.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdown)
	}()
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}

// findPDFText looks for the PDFKit helper next to the binary, then in ./bin.
func findPDFText() string {
	candidates := []string{"bin/pdftext"}
	if exe, err := os.Executable(); err == nil {
		candidates = append([]string{filepath.Join(filepath.Dir(exe), "pdftext")}, candidates...)
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	return ""
}
