package lint

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UnPoilTefal/perimeter/internal/corpus"
	"github.com/UnPoilTefal/perimeter/internal/report"
)

// corpusTemp ecrit un corpus jetable. Les cas de cette suite portent sur
// l'interaction entre une note illisible et les liens qui la citent : les
// isoler evite de perturber les decomptes de testdata/corpus.
func corpusTemp(t *testing.T, files map[string]string) *corpus.Corpus {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatalf("ecriture de %s : %v", name, err)
		}
	}
	c, err := corpus.Load(dir)
	if err != nil {
		t.Fatalf("chargement : %v", err)
	}
	return c
}

func lintTemp(t *testing.T, files map[string]string) *report.Result {
	t.Helper()
	res, err := Run(corpusTemp(t, files), Options{})
	if err != nil {
		t.Fatalf("lint : %v", err)
	}
	return res
}

// note rend une note valide qui cite les cibles demandees.
func note(name string, cites ...string) string {
	b := &strings.Builder{}
	b.WriteString("---\nname: " + name + "\n")
	b.WriteString("description: \"une description assez longue pour ne pas declencher les autres regles\"\n")
	b.WriteString("metadata:\n  type: reference\n  modified: 2026-09-01\n---\n\nUn fait.\n")
	for _, c := range cites {
		b.WriteString("\nVoir [[" + c + "]].\n")
	}
	return b.String()
}

func findings(res *report.Result, rule string) []report.Finding {
	var out []report.Finding
	for _, f := range res.Findings {
		if f.Rule == rule {
			out = append(out, f)
		}
	}
	return out
}

// T1 — le coeur de l'issue #30. Une note illisible n'est pas une cible
// absente : le constat appartient a la note en defaut, pas a chacune de
// celles qui la citent. Un probleme, un signal.
func TestWikilinkNAmplifiePasUneNoteNonAnalysable(t *testing.T) {
	res := lintTemp(t, map[string]string{
		"cible.md": "name: cible\npas de frontmatter du tout\n",
		"a.md":     note("a", "cible"),
		"b.md":     note("b", "cible"),
		"c.md":     note("c", "cible"),
	})

	got := findings(res, "wikilink")
	if len(got) != 1 {
		t.Fatalf("3 notes citent une cible illisible : 1 constat attendu, %d obtenus : %v", len(got), got)
	}
	f := got[0]
	if f.File != "cible.md" {
		t.Errorf("le constat doit porter sur la note en defaut, obtenu %s", f.File)
	}
	if !strings.Contains(f.Message, "analyse") {
		t.Errorf("le message doit dire que la cible ne s'analyse pas, obtenu : %s", f.Message)
	}
	if !strings.Contains(f.Message, "3") {
		t.Errorf("le message doit porter le nombre de liens entrants, obtenu : %s", f.Message)
	}
}

// T2 — garde de non-regression : le correctif ne doit pas avaler les liens
// reellement casses, qui restent imputables a la note citante.
func TestWikilinkSignaleToujoursUneCibleReellementAbsente(t *testing.T) {
	res := lintTemp(t, map[string]string{
		"a.md": note("a", "fantome"),
		"b.md": note("b", "fantome"),
	})

	got := findings(res, "wikilink")
	if len(got) != 2 {
		t.Fatalf("2 liens vers une cible inexistante : 2 constats attendus, %d obtenus : %v", len(got), got)
	}
	for _, f := range got {
		if !strings.Contains(f.Message, "n'a pas de cible") {
			t.Errorf("message attendu inchange pour une cible absente, obtenu : %s", f.Message)
		}
	}
}

// T3 — les deux cas coexistent dans une meme note sans se confondre.
func TestWikilinkDistingueLesDeuxCasDansUneMemeNote(t *testing.T) {
	res := lintTemp(t, map[string]string{
		"cible.md": "name: cible\npas de frontmatter du tout\n",
		"a.md":     note("a", "cible", "fantome"),
	})

	par := map[string]string{}
	for _, f := range findings(res, "wikilink") {
		par[f.File] = f.Message
	}
	if len(par) != 2 {
		t.Fatalf("un constat par cause attendu (2), obtenus %d : %v", len(par), par)
	}
	if m := par["a.md"]; !strings.Contains(m, "fantome") {
		t.Errorf("a.md doit porter le lien reellement casse, obtenu : %s", m)
	}
	if m := par["cible.md"]; !strings.Contains(m, "analyse") {
		t.Errorf("cible.md doit porter le defaut d'analyse, obtenu : %s", m)
	}
}

// T4 — la question de portee de l'issue. index-orphan indexe par chemin, pas
// par name : il doit etre indifferent au fait qu'une note s'analyse ou non.
func TestIndexOrphanIndifferentAuParse(t *testing.T) {
	casse := "name: cible\npas de frontmatter du tout\n"

	cite := lintTemp(t, map[string]string{
		"MEMORY.md": "# Index\n\n- [Cible](cible.md) — citee\n",
		"cible.md":  casse,
	})
	if n := len(findings(cite, "index-orphan")); n != 0 {
		t.Errorf("une note illisible mais citee dans l'index n'est pas orpheline, %d constat(s)", n)
	}

	absente := lintTemp(t, map[string]string{
		"MEMORY.md": "# Index\n\n- [Autre](autre.md) — sans rapport\n",
		"cible.md":  casse,
	})
	if n := len(findings(absente, "index-orphan")); n != 1 {
		t.Errorf("une note illisible et absente de l'index reste orpheline, %d constat(s)", n)
	}
}
