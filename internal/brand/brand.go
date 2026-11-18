// Package brand is the app's name, in the one place the Go code reads it.
// The scripts, the Makefile and the Mac app read scripts/brand.sh, which a
// test keeps equal to these constants.
package brand

const (
	// Name is what the user reads: windows, menus, notifications.
	Name = "Kanso"
	// ID names the binaries, the cache and log files.
	ID = "kanso"
	// DataDir is the hidden folder, inside the synced folder, that holds the
	// database and the user's own vendors.
	DataDir = ".kanso"
	// BundleID identifies the Mac app; the LaunchAgent is BundleID + ".server".
	BundleID = "io.github.coddyum.kanso"
)
