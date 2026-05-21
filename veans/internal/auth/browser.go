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
// github.com/pkg/browser, which handles macOS / Linux / Windows / WSL and
// properly detaches the child process — so we don't repeat the
// kill-the-launcher-before-it-forks bug we had with a hand-rolled exec.

import (
	"context"

	"github.com/pkg/browser"
)

func osOpen(_ context.Context, url string) error {
	return browser.OpenURL(url)
}
