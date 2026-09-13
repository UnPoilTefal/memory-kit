//go:build unix

package verify

import (
	"os/exec"
	"syscall"
)

// isolateProcessGroup place la commande dans son propre groupe de processus,
// et fait porter l'annulation sur le groupe entier.
//
// Sans cela, le delai n'est pas tenu des qu'une commande lance un enfant :
// sur la plupart des shells Linux, « sh -c » fork au lieu d'exec, si bien que
// tuer le shell laisse l'enfant vivant — et tant qu'il tient le tube de
// sortie, la lecture du resultat bloque jusqu'a ce qu'il termine de lui-meme.
func isolateProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
