package workspace

import "testing"

// TestHome redirects the unified config file into a temp directory.
func TestHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("BABYSIT_HOME", home)
	t.Setenv("BABYSIT_STATE_DIR", home)
	return home
}
