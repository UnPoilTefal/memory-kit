package perimeter

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// write depose un registre temporaire et rend son chemin.
func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), File)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const complet = `
version: 1
roles:
  intention.spec:        { source: specs }
  intention.tickets:     { source: tickets }
  contrainte.decisions:  { source: adr }
  contrainte.memoire:    { source: memoire }
  etat.declare:          { source: depots }
  etat.reel:             { source: cluster }
sources:
  specs:    { adapter: files,  reliability: measured, probe: { cmd: "true" } }
  tickets:  { adapter: github, reliability: measured, endpoint: "org/produit", credential: "env:GITHUB_TOKEN",
              probe: { cmd: "echo ${endpoint}" }, query: { cmd: "echo ${endpoint} ${arg}" } }
  adr:      { adapter: git,    reliability: declared, probe: { cmd: "true" } }
  memoire:  { adapter: files,  reliability: declared, probe: { cmd: "true" } }
  depots:   { adapter: git,    reliability: declared, probe: { cmd: "true" } }
  cluster:  { adapter: http,   reliability: measured, probe: { cmd: "true" } }
`

func issueText(is []Issue) string {
	var b strings.Builder
	for _, i := range is {
		b.WriteString(i.String())
		b.WriteString("\n")
	}
	return b.String()
}

// Test d'acceptation 1 — un role non pourvu est une carence nommee, pas un
// silence. C'est la condition E1 du demarrage.
func TestRoleNonPourvuEstUnConstat(t *testing.T) {
	body := strings.Replace(complet, "  etat.reel:             { source: cluster }\n", "", 1)
	reg, err := Load(write(t, body))
	if err != nil {
		t.Fatalf("chargement : %v", err)
	}
	got := issueText(reg.Check())
	if !strings.Contains(got, "etat.reel") || !strings.Contains(got, "non pourvu") {
		t.Errorf("le role manquant devait etre nomme, obtenu :\n%s", got)
	}
}

func TestRoleRenvoyantAUneSourceAbsente(t *testing.T) {
	body := strings.Replace(complet, "etat.reel:             { source: cluster }", "etat.reel:             { source: fantome }", 1)
	reg, err := Load(write(t, body))
	if err != nil {
		t.Fatal(err)
	}
	got := issueText(reg.Check())
	if !strings.Contains(got, "fantome") {
		t.Errorf("la source inexistante devait etre nommee, obtenu :\n%s", got)
	}
}

func TestRegistreCompletNeProduitAucunConstat(t *testing.T) {
	reg, err := Load(write(t, complet))
	if err != nil {
		t.Fatal(err)
	}
	if is := reg.Check(); len(is) != 0 {
		t.Errorf("registre complet, constats inattendus :\n%s", issueText(is))
	}
}

// Test d'acceptation 2 — une source sans sonde se degrade en silence.
func TestSourceSansSondeEstRefusee(t *testing.T) {
	body := strings.Replace(complet, `specs:    { adapter: files,  reliability: measured, probe: { cmd: "true" } }`,
		`specs:    { adapter: files,  reliability: measured }`, 1)
	if _, err := Load(write(t, body)); err == nil {
		t.Fatal("une source sans sonde doit etre refusee par le schema")
	}
}

// Test d'acceptation 6 — un adaptateur inconnu echoue nommement.
func TestAdaptateurInconnuEstNomme(t *testing.T) {
	body := strings.Replace(complet, "adapter: http,   reliability: measured", "adapter: telepathie, reliability: measured", 1)
	_, err := Load(write(t, body))
	if err == nil {
		t.Fatal("un adaptateur inconnu doit etre refuse")
	}
	if !strings.Contains(err.Error(), "adapter") {
		t.Errorf("l'erreur doit designer le champ fautif, obtenu : %v", err)
	}
}

// Test d'acceptation 4 — le registre ne porte que des references de
// credential, jamais des valeurs.
func TestCredentialSansSchemeEstRefuse(t *testing.T) {
	body := strings.Replace(complet, `credential: "env:GITHUB_TOKEN"`, `credential: "ghp_AAAABBBBCCCCDDDDEEEEFFFFGGGG1234"`, 1)
	if _, err := Load(write(t, body)); err == nil {
		t.Fatal("un credential sans scheme de reference doit etre refuse")
	}
}

func TestCredentialAvecSchemeMaisValeurEstSignale(t *testing.T) {
	body := strings.Replace(complet, `credential: "env:GITHUB_TOKEN"`, `credential: "env:ghp_AAAABBBBCCCCDDDDEEEEFFFFGGGG1234"`, 1)
	reg, err := Load(write(t, body))
	if err != nil {
		t.Fatalf("le scheme est valide, le chargement doit passer : %v", err)
	}
	got := issueText(reg.Check())
	if !strings.Contains(got, "reference") {
		t.Errorf("une valeur deguisee en reference devait etre signalee, obtenu :\n%s", got)
	}
}

// Test d'acceptation 5 — la boite ne recopie aucune source. L'invariant est
// applique par le schema : il n'existe aucun champ ou declarer une copie.
func TestChampDeCopieLocaleEstRefuseParLeSchema(t *testing.T) {
	for _, champ := range []string{"cache", "mirror", "sync", "local_copy"} {
		body := strings.Replace(complet, `adr:      { adapter: git,    reliability: declared, probe: { cmd: "true" } }`,
			`adr:      { adapter: git, reliability: declared, `+champ+`: /tmp/adr, probe: { cmd: "true" } }`, 1)
		if _, err := Load(write(t, body)); err == nil {
			t.Errorf("le champ %q declare une copie locale et doit etre refuse", champ)
		}
	}
}

func TestSourceOrphelineEstSignalee(t *testing.T) {
	body := strings.TrimRight(complet, "\n") + "\n  orpheline: { adapter: files, reliability: declared, probe: { cmd: \"true\" } }\n"
	reg, err := Load(write(t, body))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(issueText(reg.Check()), "orpheline") {
		t.Error("une source rattachee a aucun role devait etre signalee")
	}
}

// Resolve substitue endpoint et arg, et refuse nommement ce qu'il ne sait pas faire.
func TestResolveSubstitueEndpointEtArg(t *testing.T) {
	reg, err := Load(write(t, complet))
	if err != nil {
		t.Fatal(err)
	}
	c, err := reg.Resolve("tickets", "42")
	if err != nil {
		t.Fatal(err)
	}
	if c.Cmd != "echo org/produit 42" {
		t.Errorf("substitution attendue \"echo org/produit 42\", obtenue %q", c.Cmd)
	}
}

func TestResolveSourceInconnueEstNommee(t *testing.T) {
	reg, _ := Load(write(t, complet))
	_, err := reg.Resolve("fantome", "")
	if err == nil || !strings.Contains(err.Error(), "fantome") {
		t.Errorf("la source inconnue doit etre nommee, obtenu : %v", err)
	}
}

func TestResolveSansInterrogationEstNomme(t *testing.T) {
	reg, _ := Load(write(t, complet))
	_, err := reg.Resolve("adr", "")
	if err == nil || !strings.Contains(err.Error(), "adr") {
		t.Errorf("une source sans query doit etre refusee nommement, obtenu : %v", err)
	}
}

func TestProbeSubstitueEndpoint(t *testing.T) {
	reg, _ := Load(write(t, complet))
	c, err := reg.ProbeOf("tickets")
	if err != nil {
		t.Fatal(err)
	}
	if c.Cmd != "echo org/produit" {
		t.Errorf("sonde attendue \"echo org/produit\", obtenue %q", c.Cmd)
	}
}

// L'exemple livre doit rester valide : c'est la documentation executable du format.
func TestExempleLivreEstCoherent(t *testing.T) {
	reg, err := Load(filepath.Join("..", "..", "examples", "perimeter.yml"))
	if err != nil {
		t.Fatalf("l'exemple doit charger : %v", err)
	}
	if is := reg.Check(); len(is) != 0 {
		t.Errorf("l'exemple doit etre coherent :\n%s", issueText(is))
	}
}
