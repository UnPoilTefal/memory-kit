package lint

import (
	"testing"

	"github.com/UnPoilTefal/perimeter/internal/corpus"
	"github.com/UnPoilTefal/perimeter/internal/report"
)

func sansDate(nom string) string {
	return "---\nname: " + nom + "\ndescription: \"Un fait parfaitement enonce, mais que rien ne permet de faire vieillir\"\n" +
		"metadata:\n  type: reference\n---\n\nUn fait.\n"
}

func avec(nom, champ, valeur string) string {
	return "---\nname: " + nom + "\ndescription: \"Un fait parfaitement enonce, et rattache a une date exploitable\"\n" +
		"metadata:\n  type: reference\n  " + champ + ": " + valeur + "\n---\n\nUn fait.\n"
}

func constatsDate(t *testing.T, fichiers map[string]string, regle func(*corpus.Config)) []report.Finding {
	t.Helper()
	c := corpusTemp(t, fichiers)
	if regle != nil {
		regle(c.Config)
	}
	res, err := Run(c, Options{})
	if err != nil {
		t.Fatal(err)
	}
	return findings(res, "staleness")
}

// E1 — par defaut, une note sans date est une erreur. Un avertissement
// n'empeche rien : c'est ce qui a laisse 18 % d'un corpus reel hors de portee
// de toute peremption.
func TestSansDateEstUneErreurParDefaut(t *testing.T) {
	got := constatsDate(t, map[string]string{"a.md": sansDate("a")}, nil)
	if len(got) != 1 {
		t.Fatalf("1 constat attendu, %d obtenus : %v", len(got), got)
	}
	if got[0].Severity != report.Error {
		t.Errorf("severite attendue erreur, obtenue %v — un avertissement n'empeche rien", got[0].Severity)
	}
}

// E2 — l'echappatoire est explicite. Un corpus qui ne peut pas s'y plier le
// declare, plutot que de subir une regle qu'il devra desactiver en entier.
func TestRequireDateFauxRetombeEnAvertissement(t *testing.T) {
	got := constatsDate(t, map[string]string{"a.md": sansDate("a")},
		func(cfg *corpus.Config) { cfg.Policy.RequireDate = false })
	if len(got) != 1 {
		t.Fatalf("le constat reste emis, %d obtenus", len(got))
	}
	if got[0].Severity != report.Warn {
		t.Errorf("severite attendue avertissement, obtenue %v", got[0].Severity)
	}
}

// E3 — les trois champs de date valent, pas seulement modified. Exiger
// nommement modified rejetterait une note prouvee la veille par verify.
func TestLesTroisChampsDeDateSatisfontLExigence(t *testing.T) {
	for _, cas := range []struct{ champ, valeur string }{
		{"modified", "2026-09-01"},
		{"verified_at", "2026-09-01"},
		{"review_after", "2027-01-01"},
	} {
		got := constatsDate(t, map[string]string{"a.md": avec("a", cas.champ, cas.valeur)}, nil)
		if len(got) != 0 {
			t.Errorf("%s suffit a dater une note, constats obtenus : %v", cas.champ, got)
		}
	}
}

// E4 — le defaut vaut bien true : sans ca, fermer le trou ne ferme rien.
func TestRequireDateVautVraiParDefaut(t *testing.T) {
	if !corpus.DefaultConfig().Policy.RequireDate {
		t.Error("une note qu'on ne peut pas faire vieillir est un defaut, pas un choix : le defaut doit etre true")
	}
}
