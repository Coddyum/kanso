package main

import (
	"flag"
	"fmt"
	"time"

	"github.com/coddyum/kanso/internal/demo"
)

// runDemo builds the demo folder (made-up data, dated from today) and serves
// it on its own port, beside the real app and without touching its folder or
// its settings: to look around, or to take the README's screenshots.
func runDemo(args []string) error {
	c := newCLI()
	fs := flag.NewFlagSet("demo", flag.ExitOnError)
	port := fs.Int("port", 48230, "HTTP port of the demo")
	dir := fs.String("dir", demo.Dir(c.home), "where to build the demo folder (replaced only if it is a demo)")
	_ = fs.Parse(args)
	if err := demo.Build(*dir, time.Now(), c.tr); err != nil {
		return err
	}
	c.ok("cli.demo.built", "path", *dir)
	c.say("cli.demo.open", "url", fmt.Sprintf("http://localhost:%d", *port))
	return serve([]string{"-port", fmt.Sprint(*port), "-root", *dir})
}
