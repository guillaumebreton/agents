// Package notify provides best-effort desktop notifications.
// Errors are silently ignored so that a missing notification daemon
// never breaks the agent lifecycle.
package notify

import (
	"fmt"
	"os/exec"
	"runtime"
)

// Send fires a desktop notification with the given title and body.
// It uses osascript on macOS and notify-send on Linux.
// On unsupported platforms it is a no-op.
func Send(title, body string) {
	switch runtime.GOOS {
	case "darwin":
		script := fmt.Sprintf(`display notification %q with title %q`, body, title)
		//nolint:errcheck
		exec.Command("osascript", "-e", script).Run()
	case "linux":
		//nolint:errcheck
		exec.Command("notify-send", title, body).Run()
	}
}
