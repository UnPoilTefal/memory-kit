package probediff

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/UnPoilTefal/perimeter/internal/corpus"
)

func note(verify, corps string) string {
	s := "---\nname: reference-essai\ndescription: \"une description assez longue pour satisfaire le schema du corpus\"\nmetadata:\n  type: reference\n  modified: 2026-09-01\n"
	s += verify + "---\n\n" + corps + "\n"
	return s
}

const preuveA = "  verify:\n    - cmd: \"test -e /a\"\n"
const preuveB = "  verify:\n    - cmd: \"test -e /b\"\n"

func corpusAvec(t *testing.T, contenu string) *corpus.Corpus {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "reference-essai.md"), []byte(contenu), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := corpus.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func ref(contenu string) ReadAtRef {
	return func(string) ([]byte, error) { return []byte(contenu), nil }
}

func absent() ReadAtRef {
	return func(string) ([]byte, error) { return nil, os.ErrNotExist }
}

// Test 7 — une preuve qui change est rapportee, avec l'avant et l'apres.
func TestUnePreuveModifieeEstRapportee(t *testing.T) {
	c := corpusAvec(t, note(preuveB, "Corps."))
	ch, err := Changed(c, ref(note(preuveA, "Corps.")))
	if err != nil {
		t.Fatal(err)
	}
	if len(ch) != 1 {
		t.Fatalf("1 note attendue, obtenue %d", len(ch))
	}
	if len(ch[0].Ajoutees) != 1 || len(ch[0].Retirees) != 1 {
		t.Errorf("une preuve ajoutee et une retiree attendues : %+v", ch[0])
	}
}

// Test 8 — c'est tout l'interet : une note dont seul le texte change
// n'apparait pas. CODEOWNERS fait relire le fichier entier, pas nous.
func TestUnTexteModifieSansPreuveNEstPasRapporte(t *testing.T) {
	c := corpusAvec(t, note(preuveA, "Corps profondement reecrit, plus long et different."))
	ch, err := Changed(c, ref(note(preuveA, "Corps.")))
	if err != nil {
		t.Fatal(err)
	}
	if len(ch) != 0 {
		t.Errorf("aucune note attendue, obtenue %+v", ch)
	}
}

// Reformuler le commentaire d'une preuve ne change pas ce qui s'execute.
func TestReformulerUneNoteDePreuveNEstPasUnChangement(t *testing.T) {
	avant := "  verify:\n    - cmd: \"test -e /a\"\n      note: ancienne formulation\n"
	apres := "  verify:\n    - cmd: \"test -e /a\"\n      note: formulation entierement revue\n"
	c := corpusAvec(t, note(apres, "Corps."))
	ch, err := Changed(c, ref(note(avant, "Corps.")))
	if err != nil {
		t.Fatal(err)
	}
	if len(ch) != 0 {
		t.Errorf("le commentaire n'execute rien : %+v", ch)
	}
}

// Changer l'attente change ce qui est prouve, donc c'est un changement.
func TestChangerLAttenteEstUnChangement(t *testing.T) {
	avant := "  verify:\n    - cmd: \"echo x\"\n      expect_stdout: \"^x$\"\n"
	apres := "  verify:\n    - cmd: \"echo x\"\n      expect_stdout: \"^.*$\"\n"
	c := corpusAvec(t, note(apres, "Corps."))
	ch, err := Changed(c, ref(note(avant, "Corps.")))
	if err != nil {
		t.Fatal(err)
	}
	if len(ch) != 1 {
		t.Errorf("changer l'attente doit etre rapporte : %+v", ch)
	}
}

func TestUneNoteNouvelleEstSignaleeCommeTelle(t *testing.T) {
	c := corpusAvec(t, note(preuveA, "Corps."))
	ch, err := Changed(c, absent())
	if err != nil {
		t.Fatal(err)
	}
	if len(ch) != 1 || !ch[0].Nouvelle {
		t.Fatalf("note nouvelle attendue : %+v", ch)
	}
	if len(ch[0].Ajoutees) != 1 {
		t.Errorf("sa preuve est nouvelle : %+v", ch[0])
	}
}

func TestUneNoteNouvelleSansPreuveNEstPasRapportee(t *testing.T) {
	c := corpusAvec(t, note("", "Corps."))
	ch, err := Changed(c, absent())
	if err != nil {
		t.Fatal(err)
	}
	if len(ch) != 0 {
		t.Errorf("rien n'execute de code : %+v", ch)
	}
}

// Une preuve adossee a une source se compare comme les autres.
func TestUnePreuveAdosseeAUneSourceSeCompare(t *testing.T) {
	avant := "  verify:\n    - source: tickets\n      arg: \"1\"\n"
	apres := "  verify:\n    - source: tickets\n      arg: \"2\"\n"
	c := corpusAvec(t, note(apres, "Corps."))
	ch, err := Changed(c, ref(note(avant, "Corps.")))
	if err != nil {
		t.Fatal(err)
	}
	if len(ch) != 1 {
		t.Errorf("changer l'argument change ce qui est interroge : %+v", ch)
	}
}

func TestResumeDitCeQuiAChange(t *testing.T) {
	c := Change{Note: "x.md", Ajoutees: []string{"a"}, Retirees: []string{"b"}, Nouvelle: true}
	r := c.Resume()
	for _, attendu := range []string{"nouvelle", "ajoutee", "retiree"} {
		if !contains(r, attendu) {
			t.Errorf("le resume doit mentionner %q : %q", attendu, r)
		}
	}
	if c.Count() != 2 {
		t.Errorf("2 preuves touchees attendues, obtenu %d", c.Count())
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
