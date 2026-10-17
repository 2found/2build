//go:build bbs_noembed

package webui

import "io/fs"

// FS never reports an embedded SPA, so callers use web/dist on disk.
func FS() (fs.FS, bool) { return nil, false }
