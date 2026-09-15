package draft

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/UnPoilTefal/perimeter/internal/corpus"
)

// corpusExistant rend un corpus de reference contre lequel juger un brouillon.
func corpusExistant(t *testing.T) *corpus.Corpus {
	t.Helper()
	dir := t.TempDir()
	fichiers := map[string]string{
		"MEMORY.md": "# Index\n\n" +
			"- [Reference: port udm](reference-port-udm.md) — le port 9 est le WAN2\n" +
			"- [Reference: kubectl](reference-kubectl-path.md) — absent du PATH hors session interactive\n",
		"reference-port-udm.md": note("reference-port-udm",
			"Le port 9 de l'UDM Pro est le WAN2, indiscernable d'un port LAN libre dans l'interface"),
		"reference-kubectl-path.md": note("reference-kubectl-path",
			"kubectl est absent du PATH en session non interactive, installe par brew et declare au Brewfile"),
	}
	for nom, corps := range fichiers {
		if err := os.WriteFile(filepath.Join(dir, nom), []byte(corps), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	c, err := corpus.Load(dir)
	if err != nil {
		t.Fatalf("chargement : %v", err)
	}
	return c
}

func note(nom, desc string) string {
	return "---\nname: " + nom + "\ndescription: \"" + desc + "\"\n" +
		"metadata:\n  type: reference\n  modified: 2026-09-01\n---\n\nUn fait verifiable.\n"
}

func verdict(t *testing.T, c *corpus.Corpus, rel, brouillon string) *Verdict {
	t.Helper()
	n, err := corpus.ParseNote(rel, []byte(brouillon))
	if err != nil {
		t.Fatalf("analyse du brouillon : %v", err)
	}
	v, err := Check(c, n, nil, Options{})
	if err != nil {
		t.Fatalf("check : %v", err)
	}
	return v
}

func regles(v *Verdict) map[string]bool {
	m := map[string]bool{}
	for _, f := range v.Findings {
		m[f.Rule] = true
	}
	return m
}

// D1 — un nom deja pris est une collision, pas une note de plus. Sans ca, le
// fichier ecrase une note existante ou lui vole ses liens entrants.
func TestCollisionDeNomEstSignalee(t *testing.T) {
	c := corpusExistant(t)
	v := verdict(t, c, "reference-port-udm.md", note("reference-port-udm", "Une autre affirmation, sur un tout autre sujet reseau"))
	if v.Collision == nil {
		t.Fatal("le nom reference-port-udm est deja pris, la collision devait etre signalee")
	}
	if v.Collision.Rel != "reference-port-udm.md" {
		t.Errorf("collision attendue sur reference-port-udm.md, obtenue %s", v.Collision.Rel)
	}
}

// D2 — le voisinage repond a « est-ce que ca existe deja ? ». Mesure : le
// signal classe correctement, mais son echelle absolue ne veut rien dire —
// donc une liste ordonnee avec son motif, jamais un verdict.
func TestVoisinageRemonteLaNoteProcheEnTete(t *testing.T) {
	c := corpusExistant(t)
	v := verdict(t, c, "reference-udm-wan2.md", note("reference-udm-wan2",
		"Sur l'UDM Pro le port 9 est un port WAN2 et non un port LAN libre"))

	if len(v.Voisins) == 0 {
		t.Fatal("un brouillon proche d'une note existante doit remonter du voisinage")
	}
	if v.Voisins[0].Note.Rel != "reference-port-udm.md" {
		t.Errorf("voisin le plus proche attendu reference-port-udm.md, obtenu %s", v.Voisins[0].Note.Rel)
	}
	if len(v.Voisins[0].Termes) == 0 {
		t.Error("le voisin doit porter son motif : les termes partages")
	}
}

// D3 — les regles qui n'ont pas de sens sur une note non encore ecrite ne
// doivent pas se declencher. Un brouillon n'est pas dans l'index : le dire
// serait un constat que rien ne permet de corriger.
func TestLesReglesDIndexNeSAppliquentPasAUnBrouillon(t *testing.T) {
	c := corpusExistant(t)
	v := verdict(t, c, "reference-nouvelle.md", note("reference-nouvelle",
		"Un fait entierement nouveau, sans rapport avec le reste du corpus de reference"))

	for _, interdite := range []string{"index-orphan", "index-dangling", "index-drift", "staleness", "staleness-budget"} {
		if regles(v)[interdite] {
			t.Errorf("la regle %s ne s'applique pas a un brouillon non ecrit", interdite)
		}
	}
}

// D4 — les regles qui portent sur la note elle-meme s'appliquent. Une
// description qui nomme le sujet au lieu d'enoncer le fait rend la note aussi
// inutile qu'une note absente de l'index.
func TestLesReglesDeNoteSAppliquent(t *testing.T) {
	c := corpusExistant(t)

	court := verdict(t, c, "reference-vague.md", note("reference-vague", "notes sur UniFi"))
	if !regles(court)["description"] {
		t.Errorf("une description qui nomme le sujet doit etre signalee, obtenu %v", court.Findings)
	}

	inconnu := verdict(t, c, "reference-type.md",
		"---\nname: reference-type\ndescription: \"Un fait parfaitement valable et suffisamment detaille\"\nmetadata:\n  type: divagation\n  modified: 2026-09-01\n---\n\nUn fait.\n")
	if !regles(inconnu)["schema"] {
		t.Errorf("un registre hors des cinq doit etre refuse par le schema, obtenu %v", inconnu.Findings)
	}
}

// D5 — la contrainte dure du lot : des propositions relues, pas des
// ecritures. Une memoire qu'un agent modifie sans revue est le mecanisme
// exact par lequel une hypothese fausse devient canon.
func TestCheckNEcritRien(t *testing.T) {
	c := corpusExistant(t)
	avant := empreinte(t, c.Root)

	verdict(t, c, "reference-nouvelle.md", note("reference-nouvelle", "Un fait entierement nouveau et parfaitement decrit ici"))
	verdict(t, c, "reference-port-udm.md", note("reference-port-udm", "Une collision volontaire avec une note existante du corpus"))

	if apres := empreinte(t, c.Root); apres != avant {
		t.Errorf("draft a modifie le corpus\navant : %s\napres : %s", avant, apres)
	}
}

func empreinte(t *testing.T, racine string) string {
	t.Helper()
	var b strings.Builder
	entrees, err := os.ReadDir(racine)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entrees {
		raw, err := os.ReadFile(filepath.Join(racine, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		b.WriteString(e.Name() + ":" + string(raw) + "\n")
	}
	return b.String()
}

// D6 — un brouillon sain ne doit produire ni collision, ni constat bloquant.
// Sans ce garde, le portillon refuserait tout et ne serait jamais utilise.
func TestUnBrouillonSainPasse(t *testing.T) {
	c := corpusExistant(t)
	v := verdict(t, c, "reference-nfs-dsm.md", note("reference-nfs-dsm",
		"La syntaxe de plage IP 192.168.x.a-b est acceptee par l'interface DSM mais jamais resolue par exportfs"))

	if v.Collision != nil {
		t.Errorf("aucune collision attendue, obtenue sur %s", v.Collision.Rel)
	}
	if v.Bloquant() {
		t.Errorf("un brouillon sain ne doit pas etre bloque, constats : %v", v.Findings)
	}
}

// D7 — les accents ne doivent pas couper le voisinage. Trouve a l'epreuve sur
// un corpus francais reel : un brouillon ecrit « verifier » ne rencontrait
// jamais « verifier » accentue dans les notes, et la note a remonter restait
// invisible.
func TestLeVoisinageReplieLesAccents(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "reference-accentuee.md"),
		[]byte(note("reference-accentuee", "Toujours vérifier la branche git courante avant d'éditer un dépôt frère")), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := corpus.Load(dir)
	if err != nil {
		t.Fatal(err)
	}

	v := verdict(t, c, "reference-sans-accent.md",
		note("reference-sans-accent", "Verifier la branche git courante avant d'editer un depot frere"))

	if len(v.Voisins) == 0 {
		t.Fatal("le brouillon sans accents doit rencontrer la note accentuee")
	}
	for _, attendu := range []string{"verifier", "branche", "depot"} {
		trouve := false
		for _, m := range v.Voisins[0].Termes {
			if m == attendu {
				trouve = true
			}
		}
		if !trouve {
			t.Errorf("%q devait figurer dans les termes communs, obtenus %v", attendu, v.Voisins[0].Termes)
		}
	}
}

// D8 — le plafond est un plafond, pas un seuil. La mesure ne soutient aucun
// seuil : c'est le cout de relecture qu'on borne, pas la pertinence.
func TestLeVoisinageEstPlafonneNonSeuille(t *testing.T) {
	c := corpusExistant(t)
	v, err := Check(c, noteParsee(t, "reference-unique.md",
		note("reference-unique", "Le port 9 kubectl PATH UDM Pro WAN2 brew session interactive")), nil, Options{Voisins: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(v.Voisins) != 1 {
		t.Fatalf("plafond a 1 : 1 voisin attendu, %d obtenus", len(v.Voisins))
	}
}

func noteParsee(t *testing.T, rel, raw string) *corpus.Note {
	t.Helper()
	n, err := corpus.ParseNote(rel, []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return n
}
