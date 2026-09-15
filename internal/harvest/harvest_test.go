package harvest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UnPoilTefal/perimeter/internal/corpus"
	"github.com/UnPoilTefal/perimeter/internal/perimeter"
)

// piege porte les deux signaux : ce qui ne marche pas, et pourquoi.
const piege = `La garde testait le code de retour de xattr, en supposant le
binaire present. Elle ne protege donc de rien sur une machine ou il est
absent, parce que l'appel de garde echoue lui-meme.`

// diff decrit ce que le commit fait — re-derivable depuis le diff, donc hors
// memoire par axiome.
const diff = `Ajoute le champ index_hook a la politique du corpus et cable la
regle correspondante. Met a jour le schema et la documentation.`

// mou ne porte qu'un signal de piege, sans le pourquoi.
const mou = `Corrige un cas ou la commande echoue.`

func registre(t *testing.T, avecDeclare bool) *perimeter.Registry {
	t.Helper()
	roles := `
  intention.spec:        { source: s }
  intention.tickets:     { source: s }
  contrainte.decisions:  { source: s }
  contrainte.memoire:    { source: s }
  etat.reel:             { source: s }
`
	if avecDeclare {
		roles += "  etat.declare:          { source: depot }\n"
	} else {
		roles += "  etat.declare:          { source: s }\n"
	}
	body := "version: 1\nroles:" + roles + `sources:
  s:     { adapter: files, reliability: declared, probe: { cmd: "true" } }
  depot: { adapter: git,   reliability: declared, endpoint: "/tmp/depot", probe: { cmd: "true" } }
`
	f := filepath.Join(t.TempDir(), "perimeter.yml")
	if err := os.WriteFile(f, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	reg, err := perimeter.Load(f)
	if err != nil {
		t.Fatalf("registre : %v", err)
	}
	return reg
}

func corpusVide(t *testing.T) *corpus.Corpus {
	t.Helper()
	c, err := corpus.Load(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func faux(elems ...Element) Lecteur {
	return func(_, _ string, _ Options) ([]Element, error) { return elems, nil }
}

func lance(t *testing.T, c *corpus.Corpus, reg *perimeter.Registry, l Lecteur, o Options) *Result {
	t.Helper()
	o.Lecteur = l
	o.AllowExec = true
	r, err := Run(c, reg, o)
	if err != nil {
		t.Fatalf("harvest : %v", err)
	}
	return r
}

// H1 — les deux signaux sont exiges. Le filtrage est volontairement agressif :
// un harvest qui propose tout ce qu'il trouve reproduit le probleme qu'il
// pretend resoudre. Mesure sur un depot reel : 69 corps portent un signal de
// piege, 38 un pourquoi, 20 les deux.
func TestLesDeuxSignauxSontExiges(t *testing.T) {
	r := lance(t, corpusVide(t), registre(t, true), faux(
		Element{Ref: "a1", Titre: "cask", Corps: piege},
		Element{Ref: "b2", Titre: "index_hook", Corps: diff},
		Element{Ref: "c3", Titre: "correctif", Corps: mou},
	), Options{})

	if len(r.Candidats) != 1 {
		t.Fatalf("1 candidat attendu, %d obtenus : %v", len(r.Candidats), refs(r))
	}
	if r.Candidats[0].Ref != "a1" {
		t.Errorf("candidat attendu a1, obtenu %s", r.Candidats[0].Ref)
	}
	if r.Candidats[0].Signal == "" {
		t.Error("un candidat doit porter la phrase qui l'a declenche")
	}
	if r.Lus != 3 {
		t.Errorf("3 elements lus attendus, %d", r.Lus)
	}
}

// H2 — tout candidat arrive en « proposed ». Un harvest qui produirait du
// canon ferait entrer en verite d'equipe ce que personne n'a relu.
func TestToutCandidatEstProposed(t *testing.T) {
	r := lance(t, corpusVide(t), registre(t, true), faux(Element{Ref: "a1", Titre: "t", Corps: piege}), Options{})
	if got := r.Candidats[0].Trust; got != "proposed" {
		t.Errorf("trust attendu \"proposed\", obtenu %q", got)
	}
}

// H3 — la porte de perimetre est structurelle : harvest ne lit que des sources
// declarees. Sans source d'etat declare exploitable, il refuse plutot que de
// deviner ou chercher.
func TestRefuseSansSourceDeclaree(t *testing.T) {
	_, err := Run(corpusVide(t), registre(t, false), Options{AllowExec: true, Lecteur: faux()})
	if err == nil {
		t.Fatal("sans source git en etat.declare, harvest doit refuser")
	}
	if !strings.Contains(err.Error(), "declare") {
		t.Errorf("le refus doit nommer le role manquant, obtenu : %v", err)
	}
}

// H4 — l'execution passe par la meme porte que le reste de l'outil. Harvest
// lance des commandes : sans --allow-exec, il refuse.
func TestRefuseSansAllowExec(t *testing.T) {
	_, err := Run(corpusVide(t), registre(t, true), Options{Lecteur: faux()})
	if err == nil || !strings.Contains(err.Error(), "allow-exec") {
		t.Fatalf("l'execution doit etre gardee par --allow-exec, obtenu : %v", err)
	}
}

// H5 — le dedoublonnage reutilise le voisinage de « draft » : un candidat qui
// redit une note existante doit la faire remonter **en tete**, pour que la
// relecture commence par la.
//
// Ce test affirmait d'abord un marqueur binaire « deja connu ». La mesure l'a
// invalide : sur 20 candidats reels, un seuil a 0,15 marquait un vrai doublon,
// un faux, et en manquait un troisieme. Le rang tient, le score ne tient pas.
func TestUnCandidatDejaConnuPorteSonVoisinage(t *testing.T) {
	dir := t.TempDir()
	note := "---\nname: reference-garde-xattr\n" +
		"description: \"la garde testait le code de retour de xattr en supposant le binaire present, elle ne protegeait donc de rien\"\n" +
		"metadata:\n  type: reference\n  modified: 2026-09-01\n---\n\nUn fait.\n"
	if err := os.WriteFile(filepath.Join(dir, "reference-garde-xattr.md"), []byte(note), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := corpus.Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	r := lance(t, c, registre(t, true), faux(Element{Ref: "a1", Titre: "garde xattr", Corps: piege}), Options{})
	if len(r.Candidats) != 1 {
		t.Fatalf("1 candidat attendu, %d", len(r.Candidats))
	}
	if len(r.Candidats[0].Voisins) == 0 {
		t.Fatal("un candidat proche d'une note existante doit porter son voisinage")
	}
	if got := r.Candidats[0].Voisins[0].Note.Rel; got != "reference-garde-xattr.md" {
		t.Errorf("la note qui redit le candidat doit sortir en tete, obtenu %s", got)
	}
}

// H6 — le plafond borne le cout de relecture, comme pour le voisinage. Un
// harvest sans plafond rend son propre resultat inexploitable.
func TestLePlafondBorneLaSortie(t *testing.T) {
	var elems []Element
	for _, ref := range []string{"a", "b", "c", "d", "e"} {
		elems = append(elems, Element{Ref: ref, Titre: "t", Corps: piege})
	}
	r := lance(t, corpusVide(t), registre(t, true), faux(elems...), Options{Limite: 2})
	if len(r.Candidats) != 2 {
		t.Fatalf("plafond a 2 : 2 candidats attendus, %d", len(r.Candidats))
	}
	if r.Retenus != 5 {
		t.Errorf("le nombre reellement retenu doit rester visible (5), obtenu %d", r.Retenus)
	}
}

// H7 — harvest ne produit que des propositions. Rien n'est ecrit, ni dans le
// corpus ni ailleurs : c'est la contrainte dure du lot.
func TestHarvestNEcritRien(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "MEMORY.md"), []byte("# Index\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := corpus.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	avant, _ := os.ReadDir(dir)
	empreinte := func() string {
		var b strings.Builder
		for _, e := range avant {
			raw, _ := os.ReadFile(filepath.Join(dir, e.Name()))
			b.WriteString(e.Name() + string(raw))
		}
		return b.String()
	}
	av := empreinte()
	lance(t, c, registre(t, true), faux(Element{Ref: "a1", Titre: "t", Corps: piege}), Options{})
	if empreinte() != av {
		t.Error("harvest a modifie le corpus")
	}
}

func refs(r *Result) []string {
	var out []string
	for _, c := range r.Candidats {
		out = append(out, c.Ref)
	}
	return out
}
