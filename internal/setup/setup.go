// Package setup installs and removes what runs Kanso on the Mac: the
// LaunchAgent that starts the server at login, and the places the app writes
// to outside the user's folder (cache, log, settings, the Mac app).
package setup

import (
	"bytes"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/coddyum/kanso/internal/brand"
	"github.com/coddyum/kanso/internal/config"
)

// Label is the LaunchAgent's name.
const Label = brand.BundleID + ".server"

// Paths are the places Kanso writes to outside the user's folder.
type Paths struct {
	Agent  string // ~/Library/LaunchAgents/<label>.plist
	Log    string // ~/Library/Logs/kanso.log
	Cache  string // ~/Library/Caches/kanso: PDF texts, rates, logos
	Config string // ~/Library/Application Support/Kanso
	App    string // ~/Applications/Kanso.app
}

// PathsOf are the paths for the user whose home is home.
func PathsOf(home string) Paths {
	return Paths{
		Agent:  filepath.Join(home, "Library", "LaunchAgents", Label+".plist"),
		Log:    filepath.Join(home, "Library", "Logs", brand.ID+".log"),
		Cache:  filepath.Join(home, "Library", "Caches", brand.ID),
		Config: config.Dir(home),
		App:    filepath.Join(home, "Applications", brand.Name+".app"),
	}
}

// AgentPlist is the LaunchAgent: launchd starts the launcher at login, the
// launcher starts `kanso serve` and restarts it after a stop or a new build.
func AgentPlist(binDir, workDir, logPath string) []byte {
	esc := html.EscapeString
	var b bytes.Buffer
	fmt.Fprintf(&b, `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key><string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>%s</string>
        <string>serve</string>
    </array>
    <key>WorkingDirectory</key><string>%s</string>
    <key>RunAtLoad</key><true/>
    <key>KeepAlive</key><true/>
    <key>ProcessType</key><string>Background</string>
    <key>LowPriorityIO</key><true/>
    <key>StandardOutPath</key><string>%s</string>
    <key>StandardErrorPath</key><string>%s</string>
</dict>
</plist>
`, Label, esc(filepath.Join(binDir, brand.ID+"-launcher")), esc(filepath.Join(binDir, brand.ID)),
		esc(workDir), esc(logPath), esc(logPath))
	return b.Bytes()
}

func domain() string { return "gui/" + strconv.Itoa(os.Getuid()) }

// InstallAgent writes the LaunchAgent and (re)starts it.
func InstallAgent(p Paths, binDir, workDir string) error {
	if err := os.MkdirAll(filepath.Dir(p.Agent), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p.Log), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(p.Agent, AgentPlist(binDir, workDir, p.Log), 0o644); err != nil {
		return err
	}
	_ = exec.Command("launchctl", "bootout", domain()+"/"+Label).Run()
	// bootout returns before launchd lets go of the old server: a bootstrap
	// right after it fails with « 5: Input/output error ». Wait, then retry.
	err := fmt.Errorf("launchctl bootstrap: %s is still loaded", Label)
	for try := 0; try < 20; try++ {
		if try > 0 {
			time.Sleep(250 * time.Millisecond)
		}
		if AgentLoaded() {
			continue
		}
		out, e := exec.Command("launchctl", "bootstrap", domain(), p.Agent).CombinedOutput()
		if e == nil {
			return nil
		}
		err = fmt.Errorf("launchctl bootstrap: %v: %s", e, bytes.TrimSpace(out))
	}
	return err
}

// UninstallAgent stops the server and removes the LaunchAgent.
func UninstallAgent(p Paths) error {
	_ = exec.Command("launchctl", "bootout", domain()+"/"+Label).Run()
	if err := os.Remove(p.Agent); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// AgentLoaded says whether launchd runs the LaunchAgent now.
func AgentLoaded() bool {
	return exec.Command("launchctl", "print", domain()+"/"+Label).Run() == nil
}
