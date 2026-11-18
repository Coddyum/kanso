// coddyum-inc-launcher keeps coddyum-inc running for launchd: it starts the
// app, starts it again when it stops, and restarts it as soon as its binary is
// rebuilt — an update needs no manual restart.
//
// It stays the same binary from one update to the next: macOS asks once for
// the access to the Documents folder, for this launcher, and the app it starts
// inherits that access.
package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage : coddyum-inc-launcher <binaire> [arguments…]")
	}
	bin, args := os.Args[1], os.Args[2:]
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	pause := time.Second
	for {
		stamp := stampOf(bin)
		cmd := exec.Command(bin, args...)
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		started := time.Now()
		if err := cmd.Start(); err != nil {
			log.Printf("lancement impossible : %v", err)
			if wait(stop, pause) {
				return
			}
			pause = min(pause*2, 30*time.Second)
			continue
		}
		exited := make(chan error, 1)
		go func() { exited <- cmd.Wait() }()
		switch watch(bin, stamp, exited, stop) {
		case "stop":
			end(cmd, exited)
			return
		case "update":
			log.Printf("nouvelle version de %s : redémarrage", bin)
			end(cmd, exited)
			pause = time.Second
		case "exit":
			if time.Since(started) > time.Minute {
				pause = time.Second
			}
			if wait(stop, pause) {
				return
			}
			pause = min(pause*2, 30*time.Second)
		}
	}
}

// watch waits for the app to stop, for launchd to stop the launcher, or for
// the binary to change on disk and stay unchanged for a second (a finished build).
func watch(bin, stamp string, exited <-chan error, stop <-chan os.Signal) string {
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-stop:
			return "stop"
		case err := <-exited:
			log.Printf("coddyum-inc s'est arrêté : %v", err)
			return "exit"
		case <-tick.C:
			if s := stampOf(bin); s != "" && s != stamp {
				time.Sleep(time.Second)
				if stampOf(bin) == s {
					return "update"
				}
			}
		}
	}
}

// end asks the app to stop, and kills it after 5 seconds.
func end(cmd *exec.Cmd, exited <-chan error) {
	_ = cmd.Process.Signal(syscall.SIGTERM)
	select {
	case <-exited:
	case <-time.After(5 * time.Second):
		_ = cmd.Process.Kill()
		<-exited
	}
}

// wait sleeps d, or less when launchd asks the launcher to stop (true).
func wait(stop <-chan os.Signal, d time.Duration) bool {
	select {
	case <-stop:
		return true
	case <-time.After(d):
		return false
	}
}

func stampOf(path string) string {
	st, err := os.Stat(path)
	if err != nil {
		return ""
	}
	return fmt.Sprintf("%d-%d", st.ModTime().UnixNano(), st.Size())
}
