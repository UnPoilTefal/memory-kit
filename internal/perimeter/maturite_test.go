package perimeter

import (
	"strings"
	"testing"
)

func chargeur(t *testing.T, body string) *Registry {
	t.Helper()
	reg, err := Load(write(t, body))
	if err != nil {
		t.Fatalf("registre : %v", err)
	}
	return reg
}

func motifs(rs []Remarque) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.Motif)
	}
	return out
}

func contientMotif(rs []Remarque, motif string) bool {
	for _, r := range rs {
		if r.Motif == motif {
			return true
		}
	}
	return false
}

// M1 — un etat.reel declare et non mesure est l'angle mort meme que l'outil
// existe pour reveler. C'est aussi ce qui donne enfin un role au champ
// reliability, jusque-la exige par le schema et lu nulle part.
func TestEtatReelDeclareEstRemarque(t *testing.T) {
	rs := chargeur(t, complet).Maturite()
	if contientMotif(rs, "etat-reel-non-mesure") {
		t.Errorf("le registre complet a un etat.reel measured, aucune remarque attendue : %v", motifs(rs))
	}

	degrade := strings.Replace(complet,
		`cluster:  { adapter: http,   reliability: measured, probe: { cmd: "true" } }`,
		`cluster:  { adapter: http,   reliability: declared, probe: { cmd: "true" } }`, 1)
	rs = chargeur(t, degrade).Maturite()
	if !contientMotif(rs, "etat-reel-non-mesure") {
		t.Errorf("un etat.reel declare doit etre remarque, obtenu %v", motifs(rs))
	}
}

// M2 — deux familles de roles servies par la meme source : le perimetre est
// valide, mais il ne distingue plus l'intention de l'etat. Comparer le declare
// au reel n'a alors plus de sens, puisque les deux viennent du meme endroit.
func TestFamillesServiesParLaMemeSourceSontRemarquees(t *testing.T) {
	melange := strings.Replace(complet, "  etat.declare:          { source: depots }",
		"  etat.declare:          { source: specs }", 1)
	rs := chargeur(t, melange).Maturite()
	if !contientMotif(rs, "familles-confondues") {
		t.Errorf("une source servant intention et etat doit etre remarquee, obtenu %v", motifs(rs))
	}
}

// M3 — une sonde qui teste l'existence prouve que la source est la, pas
// qu'elle dit ce qu'on attend d'elle. C'est le meme motif qu'une garde qui
// invoque ce qu'elle protege.
func TestSondeDExistenceSeuleEstRemarquee(t *testing.T) {
	rs := chargeur(t, complet).Maturite()
	if !contientMotif(rs, "sonde-d-existence") {
		t.Errorf("les sondes « true » du registre de test devaient etre remarquees, obtenu %v", motifs(rs))
	}
	for _, r := range rs {
		if r.Motif == "sonde-d-existence" && r.Source == "" {
			t.Error("la remarque doit nommer la source concernee")
		}
	}
}

// M4 — pas de score. Le projet a remplace les scores de confiance par des
// tests falsifiables, et un « registre a 78 % » inviterait a optimiser le
// chiffre. Chaque remarque porte un motif nommable et sa consequence.
func TestChaqueRemarquePorteSaConsequence(t *testing.T) {
	rs := chargeur(t, complet).Maturite()
	if len(rs) == 0 {
		t.Fatal("le registre de test doit produire au moins une remarque")
	}
	for _, r := range rs {
		if r.Motif == "" || r.Message == "" || r.Consequence == "" {
			t.Errorf("remarque incomplete : %+v", r)
		}
	}
}
