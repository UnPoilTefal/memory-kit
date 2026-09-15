package corpus

import (
	"testing"

	"github.com/UnPoilTefal/perimeter/internal/perimeter"
)

// Test d'acceptation 4 — sans politique declaree, on retombe sur les defauts.
// C'est l'usage ad hoc : pointer un repertoire qui n'appartient a aucun
// perimetre.
func TestPolitiqueAbsenteRendLesDefauts(t *testing.T) {
	got := ConfigFromPolicy(nil)
	def := DefaultConfig()
	if got.Policy.MaxBodyWords != def.Policy.MaxBodyWords {
		t.Errorf("budget de mots attendu %d, obtenu %d", def.Policy.MaxBodyWords, got.Policy.MaxBodyWords)
	}
	if got.Policy.Staleness.ReviewAfterDays != def.Policy.Staleness.ReviewAfterDays {
		t.Error("budget de relecture : defaut attendu")
	}
	if len(got.Links.IgnorePrefixes) == 0 {
		t.Error("les prefixes ignores par defaut doivent survivre")
	}
}

// Test d'acceptation 1 — la politique du registre pilote reellement le lint.
func TestPolitiqueDuRegistrePiloteLeLint(t *testing.T) {
	p := &perimeter.CorpusPolicy{
		Index:        "INDEX.md",
		Types:        []string{"reference"},
		MaxBodyWords: 42,
		RequireOwner: true,
	}
	p.Staleness.ReviewAfterDays = 7
	p.Links.IgnorePrefixes = []string{"@"}
	p.Links.ExternalRoots = []string{"/ailleurs"}

	got := ConfigFromPolicy(p)
	if got.Corpus.Index != "INDEX.md" {
		t.Errorf("index attendu INDEX.md, obtenu %q", got.Corpus.Index)
	}
	if got.Policy.MaxBodyWords != 42 || !got.Policy.RequireOwner {
		t.Errorf("politique mal appliquee : %+v", got.Policy)
	}
	if got.Policy.Staleness.ReviewAfterDays != 7 {
		t.Errorf("budget de relecture attendu 7, obtenu %d", got.Policy.Staleness.ReviewAfterDays)
	}
	if len(got.Policy.Types) != 1 || got.Policy.Types[0] != "reference" {
		t.Errorf("registres attendus [reference], obtenus %v", got.Policy.Types)
	}
	if len(got.Links.ExternalRoots) != 1 {
		t.Error("les corpus voisins declares doivent etre repris")
	}
}

// Un champ laisse vide au registre ne doit pas ecraser le defaut par un zero.
func TestChampVideNEcrasePasLeDefaut(t *testing.T) {
	got := ConfigFromPolicy(&perimeter.CorpusPolicy{Index: "X.md"})
	if got.Policy.MaxBodyWords != DefaultConfig().Policy.MaxBodyWords {
		t.Errorf("budget de mots ecrase par un zero : %d", got.Policy.MaxBodyWords)
	}
	if got.Policy.Staleness.MaxStaleRatio == 0 {
		t.Error("le plafond de peremption ne doit pas tomber a zero")
	}
}
