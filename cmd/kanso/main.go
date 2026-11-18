// kanso: a small local app to follow business expenses, upcoming payments
// and invoices, kept in step with a folder of the Mac. One binary: the server
// (`kanso serve`, started by launchd) and the commands that install, update,
// check and remove it.
package main

import (
	"fmt"
	"os"
	"strings"
)

// version is the build, set by the Makefile (git describe).
var version = "dev"

func main() {
	cmd, args := "serve", os.Args[1:]
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd, args = args[0], args[1:]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve(args)
	case "init":
		err = runInit(args)
	case "update":
		err = runUpdate(args)
	case "uninstall":
		err = runUninstall(args)
	case "doctor":
		err = runDoctor(args)
	case "version":
		fmt.Println(version)
	case "help", "-h", "--help":
		fmt.Println(newCLI().tr.T("cli.usage", "version", version))
	default:
		c := newCLI()
		fmt.Fprintln(os.Stderr, c.tr.T("cli.unknown", "name", cmd))
		fmt.Fprintln(os.Stderr, c.tr.T("cli.usage", "version", version))
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "✗", err)
		os.Exit(1)
	}
}
