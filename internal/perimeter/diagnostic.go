package perimeter

import (
	"fmt"
	"os/exec"
)

// CheminAbsent rend un diagnostic pour un endpoint qui ne designe rien.
//
// L'erreur de la bibliotheque standard — « lstat /chemin: no such file or
// directory » — est exacte et inutilisable : elle ne dit ni quelle source
// declare ce chemin, ni quel role elle sert, ni quoi faire. L'outil a les
// trois informations.
func CheminAbsent(source, role, chemin string) string {
	return fmt.Sprintf("la source %q (role %s) declare %s, qui n'existe pas\n"+
		"  creer ce repertoire, ou corriger son endpoint dans le registre",
		source, role, chemin)
}

// EstDepotGit verifie qu'un endpoint est bien un depot, plutot que de laisser
// remonter « exit status 128 » — exact, et muet sur la cause.
func EstDepotGit(endpoint string) error {
	out, err := exec.Command("git", "-C", endpoint, "rev-parse", "--is-inside-work-tree").Output()
	if err != nil || string(out) == "" {
		return fmt.Errorf("%s n'est pas un depot git", endpoint)
	}
	return nil
}
