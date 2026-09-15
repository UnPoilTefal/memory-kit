package readiness

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "evaluation.yml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const prete = `
version: 1
spec: "issue #1 — un sujet clair"
tests:
  - { id: t-un, assertion: "la chose attendue se produit et se verifie", status: writable }
  - { id: t-deux, assertion: "le cas limite est couvert et observable", status: writable }
`

// Test d'acceptation 1 — le verdict n'est pas un champ. L'auteur consigne ce
// qu'il a constate ; l'outil en tire la conclusion.
func TestLeVerdictNestPasUnChampDeclarable(t *testing.T) {
	if _, err := Load(write(t, prete+"verdict: produire\n")); err == nil {
		t.Fatal("un verdict declare doit etre refuse par le schema")
	}
}

func TestSpecificationPreteDonneProduire(t *testing.T) {
	a, err := Load(write(t, prete))
	if err != nil {
		t.Fatal(err)
	}
	if d := a.Derive(nil); d.Verdict != Produire {
		t.Errorf("verdict attendu produire, obtenu %s (%v)", d.Verdict, d.Reasons)
	}
}

// Test d'acceptation 2 — une ambiguite d'intention fait rendre la main, et
// prime sur tout : aller mesurer ne sert a rien tant qu'on ignore la demande.
func TestCarenceDIntentionFaitRendreLaMain(t *testing.T) {
	body := prete + `
deficiencies:
  - { id: d-mesurable, classification: mesurable, statement: "un seuil se lit dans l'historique" }
  - { id: d-intention, classification: intention, statement: "le mot cle recouvre trois sens incompatibles" }
`
	body = strings.Replace(body, `{ id: t-deux, assertion: "le cas limite est couvert et observable", status: writable }`,
		`{ id: t-deux, assertion: "le cas limite est couvert et observable", status: blocked, blocked_by: d-intention }`, 1)
	body = strings.Replace(body, `{ id: t-un, assertion: "la chose attendue se produit et se verifie", status: writable }`,
		`{ id: t-un, assertion: "la chose attendue se produit et se verifie", status: blocked, blocked_by: d-mesurable }`, 1)
	a, err := Load(write(t, body))
	if err != nil {
		t.Fatal(err)
	}
	d := a.Derive(nil)
	if d.Verdict != RendreLaMain {
		t.Errorf("verdict attendu rendre-la-main, obtenu %s", d.Verdict)
	}
	if d.Escalation != Intention {
		t.Errorf("escalade attendue classee intention, obtenue %q", d.Escalation)
	}
}

// Test d'acceptation 3 — un savoir absent de toute source n'est pas comblable
// par l'agent : c'est une carence de couverture, et c'est elle qui mesure E3.
func TestCarenceDeConnaissanceFaitRendreLaMain(t *testing.T) {
	body := strings.Replace(prete, `{ id: t-un, assertion: "la chose attendue se produit et se verifie", status: writable }`,
		`{ id: t-un, assertion: "la chose attendue se produit et se verifie", status: blocked, blocked_by: d-savoir }`, 1) + `
deficiencies:
  - { id: d-savoir, classification: connaissance, statement: "aucune source declaree ne documente ce composant" }
`
	a, err := Load(write(t, body))
	if err != nil {
		t.Fatal(err)
	}
	d := a.Derive(nil)
	if d.Verdict != RendreLaMain || d.Escalation != Connaissance {
		t.Errorf("attendu rendre-la-main/connaissance, obtenu %s/%s", d.Verdict, d.Escalation)
	}
}

// Test d'acceptation 4 — une carence mesurable fait instruire, pas rendre la
// main. Comblee, elle ne bloque plus.
func TestCarenceMesurableFaitInstruirePuisLibere(t *testing.T) {
	base := strings.Replace(prete, `{ id: t-un, assertion: "la chose attendue se produit et se verifie", status: writable }`,
		`{ id: t-un, assertion: "la chose attendue se produit et se verifie", status: blocked, blocked_by: d-seuil }`, 1)

	a, err := Load(write(t, base+`
deficiencies:
  - { id: d-seuil, classification: mesurable, statement: "le seuil se lit dans l'historique du journal" }
`))
	if err != nil {
		t.Fatal(err)
	}
	if d := a.Derive(nil); d.Verdict != Instruire {
		t.Errorf("carence mesurable ouverte : verdict attendu instruire, obtenu %s", d.Verdict)
	}

	b, err := Load(write(t, base+`
deficiencies:
  - { id: d-seuil, classification: mesurable, statement: "le seuil se lit dans l'historique du journal", resolved: true, resolved_by: "lu : 3 echecs consecutifs" }
`))
	if err != nil {
		t.Fatal(err)
	}
	// Le test reste bloque, mais plus aucune carence ne l'explique : c'est
	// une incoherence, pas un feu vert.
	if d := b.Derive(nil); d.Verdict == Produire {
		t.Error("un test encore bloque ne doit pas donner produire")
	}
}

// Test d'acceptation 5 — une precondition mecanique empeche produire, et
// donne instruire : par construction l'agent peut aller la regler.
func TestPreconditionMecaniqueEmpecheProduire(t *testing.T) {
	a, err := Load(write(t, prete))
	if err != nil {
		t.Fatal(err)
	}
	pre := []Precondition{{Code: "fait-perime", Message: "le fait X est a relire"}}
	d := a.Derive(pre)
	if d.Verdict != Instruire {
		t.Errorf("verdict attendu instruire, obtenu %s", d.Verdict)
	}
	if len(d.Reasons) == 0 || !strings.Contains(d.Reasons[0], "relire") {
		t.Errorf("la precondition doit apparaitre dans les raisons : %v", d.Reasons)
	}
}

// Test d'acceptation 6 — omettre de classer ce qui bloque rendrait le verdict
// faussement vert. L'incoherence est donc refusee avant toute derivation.
func TestTestBloqueSansCarenceEstUneIncoherence(t *testing.T) {
	body := strings.Replace(prete, `status: writable }`, `status: blocked }`, 1)
	a, err := Load(write(t, body))
	if err != nil {
		t.Fatal(err)
	}
	issues := a.Coherence()
	if len(issues) == 0 {
		t.Fatal("un test bloque sans carence designee doit etre signale")
	}
	if !strings.Contains(strings.Join(issues, " "), "t-un") {
		t.Errorf("le test fautif doit etre nomme : %v", issues)
	}
}

func TestCarenceQuiNeBloqueRienEstSignalee(t *testing.T) {
	a, err := Load(write(t, prete+`
deficiencies:
  - { id: d-orpheline, classification: mesurable, statement: "une carence que rien ne relie a un test" }
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Coherence()) == 0 {
		t.Error("une carence ne bloquant aucun test et non resolue doit etre signalee")
	}
}

func TestCarenceRenvoyantAUneCarenceInexistante(t *testing.T) {
	body := strings.Replace(prete, `status: writable }`, `status: blocked, blocked_by: fantome }`, 1)
	a, err := Load(write(t, body))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(a.Coherence(), " "), "fantome") {
		t.Error("une carence inexistante doit etre nommee")
	}
}

// --- Journal et maturite (E3) ---

// Test d'acceptation 7 — le journal est en ajout seul.
func TestJournalEstEnAjoutSeul(t *testing.T) {
	p := filepath.Join(t.TempDir(), LedgerFile)
	for i := 0; i < 3; i++ {
		if err := Append(p, Entry{At: time.Now(), Spec: "s", Verdict: Produire}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := ReadLedger(p)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Errorf("3 passages attendus, obtenu %d", len(got))
	}
}

func TestJournalAbsentNestPasUneErreur(t *testing.T) {
	got, err := ReadLedger(filepath.Join(t.TempDir(), "inexistant.jsonl"))
	if err != nil {
		t.Fatalf("un journal absent est l'etat initial, pas une erreur : %v", err)
	}
	if len(got) != 0 {
		t.Errorf("journal vide attendu, obtenu %d", len(got))
	}
}

// Test d'acceptation 8 — le regime se deduit du journal, il n'est jamais
// configure. Fenetre incomplete : on est en demarrage par defaut.
func TestFenetreIncompleteResteEnDemarrage(t *testing.T) {
	var e []Entry
	for i := 0; i < 3; i++ {
		e = append(e, Entry{Verdict: Produire})
	}
	m := Assess(e, 5)
	if !m.Bootstrap || m.Mode != Interactif {
		t.Errorf("fenetre incomplete : demarrage/interactif attendus, obtenus %v/%s", m.Bootstrap, m.Mode)
	}
}

func TestEscaladeDeConnaissanceMaintientLeDemarrage(t *testing.T) {
	e := []Entry{
		{Verdict: Produire}, {Verdict: Instruire},
		{Verdict: RendreLaMain, Escalation: Connaissance},
		{Verdict: Produire}, {Verdict: Produire},
	}
	m := Assess(e, 5)
	if !m.Bootstrap || m.Mode != Interactif {
		t.Errorf("une escalade de connaissance maintient le demarrage, obtenu %v/%s", m.Bootstrap, m.Mode)
	}
	if m.KnowledgeEscalations != 1 {
		t.Errorf("1 escalade de connaissance attendue, comptee %d", m.KnowledgeEscalations)
	}
}

// Une escalade d'intention est le regime normal : elle ne retient pas en
// demarrage. C'est toute la distinction entre les deux classifications.
func TestEscaladeDIntentionNeRetientPasEnDemarrage(t *testing.T) {
	e := []Entry{
		{Verdict: Produire}, {Verdict: Instruire},
		{Verdict: RendreLaMain, Escalation: Intention},
		{Verdict: RendreLaMain, Escalation: Intention},
		{Verdict: Produire},
	}
	m := Assess(e, 5)
	if m.Bootstrap || m.Mode != Lot {
		t.Errorf("l'ambiguite d'intention est le regime etabli : lot attendu, obtenu %v/%s", m.Bootstrap, m.Mode)
	}
}

// La fenetre glisse : une escalade de connaissance ancienne ne retient plus.
func TestLaFenetreGlisse(t *testing.T) {
	e := []Entry{{Verdict: RendreLaMain, Escalation: Connaissance}}
	for i := 0; i < 5; i++ {
		e = append(e, Entry{Verdict: Produire})
	}
	if m := Assess(e, 5); m.Bootstrap {
		t.Error("une escalade sortie de la fenetre ne doit plus retenir en demarrage")
	}
}

// Les evaluations livrees en exemple doivent rester valides et coherentes :
// c'est la documentation executable du format.
func TestExemplesLivresSontCoherents(t *testing.T) {
	attendu := map[string]Verdict{
		"prete.yml":       Produire,
		"a-instruire.yml": Instruire,
		"a-rendre.yml":    RendreLaMain,
	}
	for nom, want := range attendu {
		a, err := Load(filepath.Join("..", "..", "examples", "readiness", nom))
		if err != nil {
			t.Errorf("%s doit charger : %v", nom, err)
			continue
		}
		if issues := a.Coherence(); len(issues) > 0 {
			t.Errorf("%s incoherent : %v", nom, issues)
		}
		if got := a.Derive(nil).Verdict; got != want {
			t.Errorf("%s : verdict attendu %s, obtenu %s", nom, want, got)
		}
	}
}

// --- la resolution doit porter sa preuve (#25) ---

const avecResolution = `
version: 1
spec: "issue #1 — une carence comblee"
tests:
  - { id: t-un, assertion: "la chose attendue se produit et se verifie", status: writable }
deficiencies:
  - id: d-seuil
    classification: mesurable
    statement: "le seuil n'est pas connu, il se lit dans l'historique"
    resolved: true
    resolved_by: "lu dans l'historique : 3 echecs consecutifs"
`

// Une resolution declaree sans dire ce qui a ete fait n'est pas verifiable,
// meme par un humain.
func TestUneResolutionSansEnonceEstUneIncoherence(t *testing.T) {
	body := strings.Replace(avecResolution, `    resolved_by: "lu dans l'historique : 3 echecs consecutifs"`+"\n", "", 1)
	a, err := Load(write(t, body))
	if err != nil {
		t.Fatal(err)
	}
	issues := a.Coherence()
	if len(issues) == 0 {
		t.Fatal("une resolution sans resolved_by doit etre signalee")
	}
	if !strings.Contains(strings.Join(issues, " "), "d-seuil") {
		t.Errorf("la carence fautive doit etre nommee : %v", issues)
	}
}

// Une resolution seulement affirmee ne bloque pas, mais elle est comptee :
// c'est ce comptage qui, accumule, montre qu'une porte est contournee.
func TestUneResolutionAffirmeeEstCompteeSansBloquer(t *testing.T) {
	a, err := Load(write(t, avecResolution))
	if err != nil {
		t.Fatal(err)
	}
	d := a.Derive(nil)
	if d.Verdict != Produire {
		t.Errorf("verdict attendu produire, obtenu %s (%v)", d.Verdict, d.Reasons)
	}
	if d.ResolutionsNonProuvees != 1 {
		t.Errorf("1 resolution non prouvee attendue, comptee %d", d.ResolutionsNonProuvees)
	}
	if !strings.Contains(strings.Join(d.Reasons, " "), "sans preuve rejouable") {
		t.Errorf("la raison doit le dire : %v", d.Reasons)
	}
}

func TestUneResolutionProuveeNEstPasComptee(t *testing.T) {
	body := avecResolution + `    resolved_proof:
      cmd: "echo 3"
      expect_stdout: "^3$"
`
	a, err := Load(write(t, body))
	if err != nil {
		t.Fatal(err)
	}
	d := a.Derive(nil)
	if d.ResolutionsNonProuvees != 0 {
		t.Errorf("une resolution prouvee ne doit pas etre comptee, obtenu %d", d.ResolutionsNonProuvees)
	}
	if !a.Deficiencies[0].Prouvee() {
		t.Error("la carence porte une preuve, Prouvee() doit le dire")
	}
}

// Une preuve ne peut pas etre a la fois une commande et un renvoi a une source.
func TestUnePreuveDeResolutionEstCommandeOuSourceMaisPasLesDeux(t *testing.T) {
	body := avecResolution + `    resolved_proof:
      cmd: "echo 3"
      source: tickets
`
	if _, err := Load(write(t, body)); err == nil {
		t.Fatal("cmd et source ensemble doivent etre refuses par le schema")
	}
}

// Quand la carence est comblee mais le test toujours bloque, la raison doit le
// dire : il reste a ecrire le test, ce n'est pas une carence de plus.
func TestUnTestBloqueParUneCarenceCombleeDitQuIlResteAEcrire(t *testing.T) {
	body := strings.Replace(avecResolution,
		"{ id: t-un, assertion: \"la chose attendue se produit et se verifie\", status: writable }",
		"{ id: t-un, assertion: \"la chose attendue se produit et se verifie\", status: blocked, blocked_by: d-seuil }", 1)
	a, err := Load(write(t, body))
	if err != nil {
		t.Fatal(err)
	}
	d := a.Derive(nil)
	if d.Verdict != Instruire {
		t.Errorf("verdict attendu instruire, obtenu %s", d.Verdict)
	}
	joint := strings.Join(d.Reasons, " ")
	if !strings.Contains(joint, "reste a ecrire") {
		t.Errorf("la raison doit dire que le test reste a ecrire : %v", d.Reasons)
	}
	if strings.Contains(joint, "sans carence qui les explique") {
		t.Errorf("la carence existe et est comblee, le message ne doit pas dire l'inverse : %v", d.Reasons)
	}
}

// Le journal porte le comptage, et la maturite l'agrege sur la fenetre.
func TestLaMaturiteAgregeLesResolutionsNonProuvees(t *testing.T) {
	var e []Entry
	for i := 0; i < 5; i++ {
		n := 0
		if i%2 == 0 {
			n = 1
		}
		e = append(e, Entry{Verdict: Produire, ResolutionsNonProuvees: n})
	}
	m := Assess(e, 5)
	if m.NonProuvees != 3 {
		t.Errorf("3 resolutions non prouvees attendues sur la fenetre, obtenu %d", m.NonProuvees)
	}
}

func TestLaFenetreNAgregeQueLesPassagesRetenus(t *testing.T) {
	e := []Entry{{Verdict: Produire, ResolutionsNonProuvees: 9}}
	for i := 0; i < 5; i++ {
		e = append(e, Entry{Verdict: Produire})
	}
	if m := Assess(e, 5); m.NonProuvees != 0 {
		t.Errorf("le passage sorti de la fenetre ne doit plus compter, obtenu %d", m.NonProuvees)
	}
}
