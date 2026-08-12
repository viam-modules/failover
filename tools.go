//go:build tools
// +build tools

package tools

import (
	_ "github.com/edaniels/golinters/cmd/combined"
	_ "github.com/rhysd/actionlint/cmd/actionlint"
)

// golangci-lint is installed at a pinned version via `make tool-install` rather than
// tracked here. Importing it into this module pulls transitive linter deps that can
// conflict with other module requirements after SDK bumps.

// This file is used for build-time dependencies only and does not contribute to the actual application.
