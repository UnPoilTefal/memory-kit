package perimeter

import (
	"fmt"
	"strings"
)

// RoleHint decrit comment proposer une brique pour un role.
type RoleHint struct {
	Role, Question, Adapter, Endpoint string
}

// RoleHints accompagne chaque role d'une question en langue naturelle et
// d'une brique par defaut, pour que le dialogue ne suppose pas le vocabulaire.
var RoleHints = []RoleHint{
	{Role: "intention.spec", Question: "Ou vit la specification (ce qui est voulu) ?", Adapter: "files", Endpoint: "docs/specs"},
	{Role: "intention.tickets", Question: "Ou vit le suivi de la demande ?", Adapter: "github", Endpoint: "org/produit"},
	{Role: "contrainte.decisions", Question: "Ou vivent les decisions (ADR) ?", Adapter: "git", Endpoint: "docs/decisions"},
	{Role: "contrainte.memoire", Question: "Ou vit le corpus de memoire ?", Adapter: "files", Endpoint: "memory"},
	{Role: "etat.declare", Question: "Ou vit le declare (depot, manifestes, IaC) ?", Adapter: "git", Endpoint: "."},
	{Role: "etat.reel", Question: "Ou se mesure le reel (API, cluster, state) ?", Adapter: "http", Endpoint: "https://api.exemple.internal"},
}

// probeFor rend une sonde qui fonctionne pour l'adaptateur donne. Une source
// sans sonde se degrade en silence : autant en poser une d'office.
func ProbeFor(adapter, endpoint string) string {
	switch adapter {
	case "files":
		return "    probe:\n      cmd: \"test -d ${endpoint}\"\n"
	case "git":
		return "    probe:\n      cmd: \"git -C ${endpoint} rev-parse --is-inside-work-tree\"\n      expect_stdout: \"^true$\"\n"
	case "github":
		return fmt.Sprintf("    probe:\n      cmd: \"gh api repos/${endpoint} --jq .full_name\"\n      expect_stdout: %q\n", "^"+regexpQuote(endpoint)+"$")
	case "gitlab":
		return "    probe:\n      cmd: \"glab api projects/$(printf %s ${endpoint} | sed 's|/|%2F|g') --method GET\"\n"
	case "http":
		return "    probe:\n      cmd: \"curl -sf -o /dev/null -w '%{http_code}' ${endpoint}/healthz\"\n      expect_stdout: \"^200$\"\n"
	default:
		return "    probe:\n      cmd: \"true\"  # a remplacer : que prouve l'acces a cette source ?\n"
	}
}

func regexpQuote(s string) string {
	r := strings.NewReplacer(".", "\\.", "+", "\\+", "*", "\\*", "?", "\\?")
	return r.Replace(s)
}

// reliabilityFor devine le regime de fiabilite : ce qui se lit dans un fichier
// versionne porte sa fraicheur, ce qui s'interroge a distance doit se mesurer.
func ReliabilityFor(adapter string) string {
	switch adapter {
	case "files", "git":
		return "declared"
	default:
		return "measured"
	}
}

// Template rend un registre commente, pret a etre relu et ajuste.
func Template() string {
	var b strings.Builder
	b.WriteString("# Registre du perimetre — https://github.com/UnPoilTefal/perimeter\n")
	b.WriteString("# Chaque role doit etre pourvu : un role vide est une carence, pas un choix.\n")
	b.WriteString("# Une source est referencee et sondee, jamais recopiee.\n")
	b.WriteString("version: 1\n\nroles:\n")
	for _, h := range RoleHints {
		fmt.Fprintf(&b, "  %-22s { source: %s }\n", h.Role+":", strings.SplitN(h.Role, ".", 2)[1])
	}
	b.WriteString("\nsources:\n")
	for _, h := range RoleHints {
		name := strings.SplitN(h.Role, ".", 2)[1]
		fmt.Fprintf(&b, "  # %s\n  %s:\n    adapter: %s\n    endpoint: %q\n    reliability: %s\n",
			h.Question, name, h.Adapter, h.Endpoint, ReliabilityFor(h.Adapter))
		b.WriteString(ProbeFor(h.Adapter, h.Endpoint))
		if h.Role == CorpusRole {
			b.WriteString("    # Politique du corpus — lue par « perctl lint ».\n")
			b.WriteString("    corpus:\n      index: MEMORY.md\n      max_body_words: 400\n      require_owner: false\n      staleness:\n        review_after_days: 180\n        max_stale_ratio: 0.15\n")
		}
	}
	return b.String()
}
