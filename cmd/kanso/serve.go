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

	"github.com/coddyum/kanso/internal/config"
	"github.com/coddyum/kanso/internal/demo"
	"github.com/coddyum/kanso/internal/server"
	"github.com/coddyum/kanso/web"
)

// serve runs the server on the folder of the config file (-root and -port
// override it). It stops cleanly on SIGTERM, and by itself after a change of
// folder: its launcher starts it again on the new one.
func serve(args []string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("home folder: %w", err)
	}
	cfg, err := config.Load(home)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	port := fs.Int("port", cfg.Port, "HTTP port (listens on 127.0.0.1 only)")
	root := fs.String("root", cfg.Root, "the folder the app keeps in step")
	_ = fs.Parse(args)
	rootSet := false
	fs.Visit(func(f *flag.Flag) { rootSet = rootSet || f.Name == "root" })
	if cfg.Demo && !rootSet {
		// Demo mode (Settings): made-up data, rebuilt from today; the user's folder is not touched.
		*root = demo.Dir(home)
		if err := demo.Build(*root, time.Now(), newCLI().tr); err != nil {
			return err
		}
	}
	if *root == "" {
		return errors.New(newCLI().tr.T("cli.noFolder"))
	}
	if st, err := os.Stat(*root); err != nil || !st.IsDir() {
		return fmt.Errorf("folder not found: %s", *root)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	pdftext := findPDFText()
	srv, err := server.New(server.Config{
		Root: *root, CacheDir: filepath.Join(home, "Library", "Caches", "kanso"),
		PDFText: pdftext, Port: *port, Static: web.Files, Home: home, Version: version, Restart: stop,
		Demo: demo.Is(*root),
	})
	if err != nil {
		return err
	}
	httpSrv := &http.Server{
		Addr:              fmt.Sprintf("127.0.0.1:%d", *port),
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("kanso %s → http://localhost:%d  (folder: %s)", version, *port, *root)
	if pdftext == "" {
		log.Printf("pdftext not found: PDFs are listed without their amount (run `make build`)")
	}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 4*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(shutdown)
	}()
	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
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
