// Vikunja is a to-do list application to facilitate your life.
// Copyright 2018-present Vikunja and contributors. All rights reserved.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <https://www.gnu.org/licenses/>.

package auth

// osOpen launches the OS's default browser at the given URL. We delegate to
// `open` on macOS, `xdg-open` on other Unixes, and `rundll32` on Windows.

import (
	"context"
	"os/exec"
	"runtime"
)

func osOpen(_ context.Context, url string) error {
	// Intentionally use context.Background() rather than the caller's ctx.
	// Cancelling the launcher process tears down the browser handoff before
	// xdg-open / open / rundll32 have had time to fork the real browser.
	// The launcher is fire-and-forget; we reap the zombie in a goroutine so
	// it doesn't linger on the process table.
	bg := context.Background()

	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(bg, "open", url) //nolint:contextcheck // detach by design — see comment above
	case "windows":
		cmd = exec.CommandContext(bg, "rundll32", "url.dll,FileProtocolHandler", url) //nolint:contextcheck // detach by design
	default:
		cmd = exec.CommandContext(bg, "xdg-open", url) //nolint:contextcheck // detach by design
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}
