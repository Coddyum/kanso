package setup

import (
	"strings"
	"testing"
)

func TestAgentPlist(t *testing.T) {
	plist := string(AgentPlist("/opt/k&k/bin", "/opt/k&k", "/tmp/kanso.log"))
	for _, want := range []string{
		"<string>io.github.coddyum.kanso.server</string>",
		"<string>/opt/k&amp;k/bin/kanso-launcher</string>",
		"<string>/opt/k&amp;k/bin/kanso</string>",
		"<string>serve</string>",
		"<key>KeepAlive</key><true/>",
	} {
		if !strings.Contains(plist, want) {
			t.Errorf("the LaunchAgent lacks %s", want)
		}
	}
}

func TestPathsOf(t *testing.T) {
	p := PathsOf("/tmp/home")
	if p.App != "/tmp/home/Applications/Kanso.app" || p.Config != "/tmp/home/Library/Application Support/Kanso" {
		t.Errorf("PathsOf = %+v", p)
	}
}
