package lint

import (
	"strings"
	"testing"

	"github.com/UnPoilTefal/perimeter/internal/corpus"
)

// corpusRedige rend un corpus dont chaque accroche d'index est ecrite a la
// main, donc differente de la description. C'est le cas mesure sur un corpus
// reel : 104 entrees, 104 divergences, aucune fautive.
func corpusRedige(t *testing.T) *corpus.Corpus {
	t.Helper()
	return corpusTemp(t, map[string]string{
		"MEMORY.md": "# Index\n\n" +
			"- [Reference: un](reference-un.md) — le piege, formule pour l'humain qui parcourt\n" +
			"- [Reference: deux](reference-deux.md) — l'autre piege, dit autrement\n",
		"reference-un.md":   noteDesc("reference-un", "Le port 9 de l'UDM Pro est le WAN2, indiscernable d'un port LAN libre"),
		"reference-deux.md": noteDesc("reference-deux", "kubectl est absent du PATH en session non interactive"),
	})
}

func noteDesc(name, desc string) string {
	return "---\nname: " + name + "\ndescription: \"" + desc + "\"\n" +
		"metadata:\n  type: reference\n  modified: 2026-09-01\n---\n\nUn fait.\n"
}

// B1 — le meme corpus, deux regimes, deux verdicts. En derived la divergence
// est un constat ; en authored la regle n'a rien a dire.
func TestIndexDriftNeSAppliquePasEnRegimeAuthored(t *testing.T) {
	c := corpusRedige(t)

	res, err := Run(c, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(findings(res, "index-drift")); n != 2 {
		t.Fatalf("regime derived (defaut) : 2 divergences attendues, %d obtenues", n)
	}

	c.Config.Policy.IndexHook = "authored"
	res, err = Run(c, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(findings(res, "index-drift")); n != 0 {
		t.Errorf("regime authored : l'accroche est ecrite a la main, 0 constat attendu, %d obtenus", n)
	}
}

// B2 — le defaut ne change pas le comportement existant : un corpus sans
// politique declaree reste en derived.
func TestIndexHookDefautDerived(t *testing.T) {
	if got := corpus.DefaultConfig().Policy.IndexHook; got != "derived" {
		t.Errorf("defaut attendu \"derived\", obtenu %q", got)
	}
}

// B3 — en authored, la regeneration doit refuser. Taire la regle sans
// desarmer la commande laisserait « --sync » ecraser d'un coup toutes les
// accroches ecrites a la main.
func TestSyncRefuseEnRegimeAuthored(t *testing.T) {
	c := corpusRedige(t)
	c.Config.Policy.IndexHook = "authored"

	if err := corpus.SyncIndexAutorise(c); err == nil {
		t.Fatal("« index --sync » doit refuser en regime authored")
	} else if !strings.Contains(err.Error(), "authored") {
		t.Errorf("le refus doit nommer le regime, obtenu : %v", err)
	}

	c.Config.Policy.IndexHook = "derived"
	if err := corpus.SyncIndexAutorise(c); err != nil {
		t.Errorf("en regime derived la regeneration reste permise, obtenu : %v", err)
	}
}
