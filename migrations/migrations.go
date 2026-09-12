package migrations

import "embed"

// holds sql migrations compiled into the binary, so a deployed container
// never depends on files being shipped alongside it.
//
//go:embed *.sql
var FS embed.FS
