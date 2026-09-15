package perimeter

import (
	"path/filepath"
	"strings"
	"testing"
)

// D1 — un chemin declare qui n'existe pas doit nommer la source, le role
// qu'elle sert, et le chemin resolu. L'erreur brute de la bibliotheque
// standard laisse tout le diagnostic a l'utilisateur alors que l'outil a
// l'information.
func TestCorpusAbsentEstDiagnostique(t *testing.T) {
	reg := chargeur(t, complet)
	manquant := filepath.Join(t.TempDir(), "memory")

	msg := CheminAbsent("memoire", "contrainte.memoire", manquant)
	for _, attendu := range []string{"memoire", "contrainte.memoire", manquant} {
		if !strings.Contains(msg, attendu) {
			t.Errorf("le diagnostic doit citer %q, obtenu :\n%s", attendu, msg)
		}
	}
	if !strings.Contains(msg, "endpoint") {
		t.Error("le diagnostic doit proposer de corriger l'endpoint")
	}
	_ = reg
}

// D2 — un endpoint qui n'est pas un depot git doit etre nomme comme tel,
// plutot que de laisser remonter le code de sortie de git.
func TestEndpointNonGitEstDiagnostique(t *testing.T) {
	dir := t.TempDir()
	if err := EstDepotGit(dir); err == nil {
		t.Fatal("un repertoire quelconque n'est pas un depot git")
	} else {
		for _, attendu := range []string{dir, "git"} {
			if !strings.Contains(err.Error(), attendu) {
				t.Errorf("le diagnostic doit citer %q, obtenu : %v", attendu, err)
			}
		}
		if strings.Contains(err.Error(), "exit status") {
			t.Errorf("le code de sortie brut ne doit pas remonter : %v", err)
		}
	}
}
