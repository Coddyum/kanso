// coddyum-inc: a tiny local web app to follow business expenses, upcoming
// payments and invoices, kept in sync with a folder of the Mac (~/Documents/Inc).
package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

//go:embed web
var webFiles embed.FS

func main() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("home folder: %v", err)
	}
	port := flag.Int("port", 48219, "port HTTP (écoute sur 127.0.0.1 uniquement)")
	root := flag.String("root", filepath.Join(home, "Documents", "Inc"), "dossier synchronisé avec l'app")
	flag.Parse()

	if st, err := os.Stat(*root); err != nil || !st.IsDir() {
		log.Fatalf("dossier introuvable : %s", *root)
	}
	app, err := newApp(*root, filepath.Join(home, "Library", "Caches", "coddyum-inc"), findPDFText())
	if err != nil {
		log.Fatal(err)
	}

	static, err := fs.Sub(webFiles, "web")
	if err != nil {
		log.Fatal(err)
	}
	mux := http.NewServeMux()
	app.routes(mux)
	mux.Handle("GET /", noCache(http.FileServerFS(static)))

	srv := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", *port),
		Handler:           guard(*port, mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("coddyum-inc → http://localhost:%d  (dossier : %s)", *port, *root)
	if app.index.pdftext == "" {
		log.Printf("pdftext introuvable : les PDF seront listés sans montant (lance `make build`)")
	}
	log.Fatal(srv.ListenAndServe())
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

// guard refuses foreign Host headers (DNS rebinding) and writes without the
// X-Inc header (a form posted by another website cannot set it).
func guard(port int, next http.Handler) http.Handler {
	allowed := map[string]bool{
		fmt.Sprintf("localhost:%d", port): true,
		fmt.Sprintf("127.0.0.1:%d", port): true,
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !allowed[r.Host] {
			http.Error(w, "hôte refusé", http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead && r.Header.Get("X-Inc") != "1" {
			http.Error(w, "en-tête X-Inc manquant", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func noCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}
