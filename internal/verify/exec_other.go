//go:build !unix

package verify

import "os/exec"

// isolateProcessGroup n'a pas d'equivalent portable hors unix : on s'en
// remet a WaitDelay, qui garantit au moins que la lecture rend la main.
func isolateProcessGroup(cmd *exec.Cmd) {}
