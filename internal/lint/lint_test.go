package lint

import (
	"testing"
	"time"

	"github.com/UnPoilTefal/perimeter/internal/corpus"
	"github.com/UnPoilTefal/perimeter/internal/report"
)

// rules rend, par regle, les fichiers concernes.
func rules(t *testing.T, res *report.Result) map[string][]string {
	t.Helper()
	m := map[string][]string{}
	for _, f := range res.Findings {
		m[f.Rule] = append(m[f.Rule], f.File)
	}
	return m
}

func run(t *testing.T) (*corpus.Corpus, *report.Result) {
	t.Helper()
	c, err := corpus.Load("testdata/corpus")
	if err != nil {
		t.Fatalf("chargement du corpus : %v", err)
	}
	res, err := Run(c, Options{Now: time.Date(2026, 9, 13, 0, 0, 0, 0, time.UTC)})
	if err != nil {
		t.Fatalf("lint : %v", err)
	}
	return c, res
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func TestIndexOrphanDetecteLesNotesInvisibles(t *testing.T) {
	_, res := run(t)
	got := rules(t, res)["index-orphan"]
	// Toutes les notes sauf reference-bon sont absentes de l'index.
	for _, want := range []string{"reference-mauvais-nom.md", "reference-secret.md", "reference-type-inconnu.md"} {
		if !contains(got, want) {
			t.Errorf("index-orphan aurait du signaler %s, a signale %v", want, got)
		}
	}
	if contains(got, "reference-bon.md") {
		t.Error("reference-bon.md est citee dans l'index, elle ne doit pas etre orpheline")
	}
}

func TestIndexDanglingDetecteLesEntreesSansFichier(t *testing.T) {
	_, res := run(t)
	if len(rules(t, res)["index-dangling"]) == 0 {
		t.Error("l'index cite reference-disparue.md, qui n'existe pas : aucun constat emis")
	}
}

func TestNameMatchDetecteLaDivergence(t *testing.T) {
	_, res := run(t)
	if !contains(rules(t, res)["name-match"], "reference-mauvais-nom.md") {
		t.Error("name-match n'a pas vu la divergence entre name et nom de fichier")
	}
}

func TestSchemaRejetteUnTypeInconnu(t *testing.T) {
	_, res := run(t)
	if !contains(rules(t, res)["schema"], "reference-type-inconnu.md") {
		t.Error("le schema doit rejeter un type hors des registres declares")
	}
}

func TestParseRapporteUneNoteSansFrontmatter(t *testing.T) {
	_, res := run(t)
	if !contains(rules(t, res)["parse"], "reference-casse.md") {
		t.Error("une note sans frontmatter doit produire un constat parse")
	}
}

func TestSecretDetecteUnJeton(t *testing.T) {
	_, res := run(t)
	if !contains(rules(t, res)["secret"], "reference-secret.md") {
		t.Error("un jeton GitHub dans le corps doit etre signale")
	}
}

func TestSecretEstUneErreurPasUnAvertissement(t *testing.T) {
	_, res := run(t)
	for _, f := range res.Findings {
		if f.Rule == "secret" && f.Severity != report.Error {
			t.Errorf("un secret doit etre une erreur, severite obtenue : %s", f.Severity)
		}
	}
}

func TestWikilinkSignaleUneCibleAbsente(t *testing.T) {
	_, res := run(t)
	if !contains(rules(t, res)["wikilink"], "reference-bon.md") {
		t.Error("[[reference-absent]] n'a pas de cible et aurait du etre signale")
	}
}

func TestWikilinkIgnoreLesPrefixesDeclares(t *testing.T) {
	c, err := corpus.Load("testdata/corpus")
	if err != nil {
		t.Fatal(err)
	}
	// Le defaut ignore les liens en "/" : ce sont des commandes, pas des notes.
	if len(c.Config.Links.IgnorePrefixes) == 0 || c.Config.Links.IgnorePrefixes[0] != "/" {
		t.Fatalf("prefixe ignore par defaut attendu \"/\", obtenu %v", c.Config.Links.IgnorePrefixes)
	}
}

func TestDisablePermetDeCouperUneRegle(t *testing.T) {
	c, err := corpus.Load("testdata/corpus")
	if err != nil {
		t.Fatal(err)
	}
	res, err := Run(c, Options{Disable: []string{"index-orphan"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(rules(t, res)["index-orphan"]) != 0 {
		t.Error("la regle desactivee a quand meme emis des constats")
	}
}

func TestExitCodeSuitLesErreurs(t *testing.T) {
	_, res := run(t)
	if res.ExitCode(false) != 1 {
		t.Error("le corpus de test contient des erreurs, le code de sortie doit etre 1")
	}
	clean := &report.Result{}
	if clean.ExitCode(false) != 0 {
		t.Error("un resultat vide doit sortir en 0")
	}
	warnOnly := &report.Result{Findings: []report.Finding{{Severity: report.Warn}}}
	if warnOnly.ExitCode(false) != 0 {
		t.Error("un avertissement seul ne doit pas faire echouer sans --strict")
	}
	if warnOnly.ExitCode(true) != 1 {
		t.Error("--strict doit faire echouer sur un avertissement")
	}
}
