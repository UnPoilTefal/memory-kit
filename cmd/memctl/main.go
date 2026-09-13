// memctl outille un corpus de memoire agent : il en verifie la structure
// (lint) et rejoue les preuves attachees aux faits (verify).
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/UnPoilTefal/memory-kit/internal/corpus"
	"github.com/UnPoilTefal/memory-kit/internal/lint"
	"github.com/UnPoilTefal/memory-kit/internal/perimeter"
	"github.com/UnPoilTefal/memory-kit/internal/report"
	"github.com/UnPoilTefal/memory-kit/internal/verify"
	"github.com/UnPoilTefal/memory-kit/schema"
	"golang.org/x/term"
)

// version est renseignee au build (-ldflags "-X main.version=…").
var version = "dev"

const usage = `memctl — outillage d'un corpus de memoire agent

  memctl lint   [chemin]   verifie la structure du corpus
  memctl verify [chemin]   rejoue les preuves attachees aux faits
  memctl index  [chemin]   compare l'index au corpus (--fix pour completer)
  memctl perimeter [reg]   valide le registre des sources du perimetre
  memctl init   [chemin]   ecrit un .memory-kit.yml
  memctl schema            ecrit le JSON Schema sur la sortie standard
  memctl version

Le chemin vaut « . » par defaut. Detail des regles et mode d'emploi : README.md
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "lint":
		err = cmdLint(os.Args[2:])
	case "verify":
		err = cmdVerify(os.Args[2:])
	case "index":
		err = cmdIndex(os.Args[2:])
	case "perimeter":
		err = cmdPerimeter(os.Args[2:])
	case "init":
		err = cmdInit(os.Args[2:])
	case "schema":
		_, err = os.Stdout.Write(schema.Memory)
	case "version":
		fmt.Printf("memctl %s\n", version)
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "commande inconnue : %s\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "memctl: %v\n", err)
		os.Exit(1)
	}
}

// fail termine avec un code sans message supplementaire : la sortie a deja
// tout dit.
func fail(code int) error {
	if code == 0 {
		return nil
	}
	os.Exit(code)
	return nil
}

func target(args []string) string {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0]
	}
	return "."
}

func isTTY() bool { return term.IsTerminal(int(os.Stdout.Fd())) }

func emit(res *report.Result, format string) error {
	return res.Write(os.Stdout, format, format == "human" && isTTY())
}

func cmdLint(args []string) error {
	fs := flag.NewFlagSet("lint", flag.ExitOnError)
	format := fs.String("format", "human", "human | json | github")
	strict := fs.Bool("strict", false, "traite les avertissements comme des erreurs")
	disable := fs.String("disable", "", "regles a desactiver, separees par des virgules")
	root := target(args)
	_ = fs.Parse(trimPositional(args))

	c, err := corpus.Load(root)
	if err != nil {
		return err
	}
	res, err := lint.Run(c, lint.Options{Disable: splitList(*disable)})
	if err != nil {
		return err
	}
	if err := emit(res, *format); err != nil {
		return err
	}
	return fail(res.ExitCode(*strict))
}

func cmdVerify(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	format := fs.String("format", "human", "human | json | github")
	allow := fs.Bool("allow-exec", false, "autorise l'execution des commandes declarees (obligatoire)")
	write := fs.Bool("write", false, "inscrit verified_at et verify_status dans les notes")
	only := fs.String("only", "", "ne verifie que les notes dont le nom contient cette chaine")
	timeout := fs.Duration("timeout", 0, "delai par commande (defaut : celui du corpus)")
	regPath := fs.String("perimeter", "", "registre des sources, pour les preuves adossees a une source")
	probes := fs.Bool("probe-sources", false, "rejoue aussi la sonde de chaque source declaree")
	root := target(args)
	_ = fs.Parse(trimPositional(args))

	c, err := corpus.Load(root)
	if err != nil {
		return err
	}
	var reg *perimeter.Registry
	if *regPath != "" {
		reg, err = perimeter.Load(*regPath)
		if err != nil {
			return err
		}
	}
	res, outcomes, err := verify.Run(c, verify.Options{
		AllowExec: *allow, Write: *write, Only: *only, Timeout: *timeout,
		Registry: reg, ProbeSources: *probes,
	})
	if errors.Is(err, verify.ErrExecRefused) {
		n, _ := res.Stats["verifiable"].(int)
		fmt.Fprintf(os.Stderr, "\n  Ces commandes sont du code executable declare dans des fichiers\n  markdown : ne les lancer que sur un corpus dont on relit les\n  contributions (CODEOWNERS)\n\n") //nolint:errcheck // sortie terminal
		return fmt.Errorf("%d notes portent des preuves : %w", n, err)
	}
	if err != nil {
		return err
	}
	if *format == "json" {
		res.Stats["outcomes"] = outcomes
	}
	if err := emit(res, *format); err != nil {
		return err
	}
	if *format == "human" {
		v, _ := res.Stats["verifiable"].(int)
		cov, _ := res.Stats["coverage"].(float64)
		fmt.Printf("%d notes portent une preuve (%.0f%% du corpus)\n", v, cov*100)
	}
	return fail(res.ExitCode(false))
}

func cmdIndex(args []string) error {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	fix := fs.Bool("fix", false, "ajoute a l'index les notes manquantes")
	sync := fs.Bool("sync", false, "regenere les accroches a partir des descriptions")
	root := target(args)
	_ = fs.Parse(trimPositional(args))

	c, err := corpus.Load(root)
	if err != nil {
		return err
	}
	if !c.HasIndex {
		return fmt.Errorf("aucun index lisible en %s", filepath.Join(c.Root, c.Config.Corpus.Index))
	}
	cited := map[string]bool{}
	for _, t := range c.IndexTargets {
		cited[t] = true
	}
	var missing []*corpus.Note
	for _, n := range c.Notes {
		if !cited[n.Rel] {
			missing = append(missing, n)
		}
	}
	if *sync {
		return syncIndex(c)
	}
	if len(missing) == 0 {
		fmt.Printf("index complet : %d notes, %d entrees\n", len(c.Notes), len(c.IndexTargets))
		return nil
	}
	if !*fix {
		fmt.Printf("%d notes absentes de l'index :\n", len(missing))
		for _, n := range missing {
			fmt.Printf("  %s\n", n.Rel)
		}
		fmt.Println("\nrelancer avec --fix pour les ajouter (les intitules restent a relire)")
		return fail(1)
	}

	raw, err := os.ReadFile(c.IndexPath)
	if err != nil {
		return err
	}
	var b strings.Builder
	b.Write(raw)
	if !strings.HasSuffix(string(raw), "\n") {
		b.WriteString("\n")
	}
	for _, n := range missing {
		b.WriteString(n.IndexLine("") + "\n")
	}
	if err := os.WriteFile(c.IndexPath, []byte(b.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("%d entrees ajoutees a %s — a relire : le titre et l'accroche viennent du frontmatter\n",
		len(missing), c.Config.Corpus.Index)
	return nil
}

// syncIndex reecrit les accroches de l'index a partir des descriptions, en
// conservant les intitules, qui eux sont ecrits a la main.
func syncIndex(c *corpus.Corpus) error {
	raw, err := os.ReadFile(c.IndexPath)
	if err != nil {
		return err
	}
	byRel := c.ByRel()
	lines := strings.Split(string(raw), "\n")
	changed := 0
	for _, e := range c.IndexEntries {
		n, ok := byRel[e.Target]
		if !ok || n.ParseErr != nil {
			continue
		}
		want := n.IndexLine(e.Title)
		if lines[e.Line-1] == want {
			continue
		}
		lines[e.Line-1] = want
		changed++
	}
	if changed == 0 {
		fmt.Println("accroches deja alignees sur les descriptions")
		return nil
	}
	if err := os.WriteFile(c.IndexPath, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return err
	}
	fmt.Printf("%d accroches regenerees dans %s\n", changed, c.Config.Corpus.Index)
	return nil
}

// cmdPerimeter valide le registre : couverture des roles, sources
// referencees, adaptateurs connus, credentials en reference. C'est la
// condition d'entree du demarrage — tant qu'elle n'est pas tenue, le
// perimetre n'est pas decrit.
func cmdPerimeter(args []string) error {
	fs := flag.NewFlagSet("perimeter", flag.ExitOnError)
	root := target(args)
	_ = fs.Parse(trimPositional(args))
	if root == "." {
		root = perimeter.File
	}

	reg, err := perimeter.Load(root)
	if err != nil {
		return err
	}
	issues := reg.Check()

	fmt.Printf("%s — %d sources, %d roles sur %d pourvus\n",
		root, len(reg.Sources), len(Roles(reg)), len(perimeter.Roles))

	if len(issues) == 0 {
		fmt.Println("\n✓ registre coherent : chaque role est pourvu et chaque source sondable")
		fmt.Println("  la sonde elle-meme se rejoue avec : memctl verify <corpus> --perimeter " + root + " --probe-sources --allow-exec")
		return nil
	}
	fmt.Printf("\n%d constats :\n", len(issues))
	for _, i := range issues {
		fmt.Printf("  - %s\n", i)
	}
	return fail(1)
}

// Roles rend les roles effectivement pourvus par une source existante.
func Roles(reg *perimeter.Registry) []string {
	var filled []string
	for _, role := range perimeter.Roles {
		b, ok := reg.RoleMap[role]
		if !ok || b.Source == "" {
			continue
		}
		if _, ok := reg.Sources[b.Source]; ok {
			filled = append(filled, role)
		}
	}
	return filled
}

func cmdInit(args []string) error {
	root := target(args)
	path := filepath.Join(root, corpus.ConfigFile)
	if _, err := os.Stat(path); err == nil {
		return fmt.Errorf("%s existe deja", path)
	}
	if err := os.WriteFile(path, []byte(defaultConfigYAML), 0o644); err != nil {
		return err
	}
	fmt.Printf("ecrit : %s\n", path)
	return nil
}

const defaultConfigYAML = `# Configuration d'un corpus de memoire — https://github.com/UnPoilTefal/memory-kit
version: 1

corpus:
  path: .
  # L'index est le routeur du rappel : une note qui n'y figure pas est
  # ecrite mais jamais lue. Mettre "" si le corpus n'en a pas.
  index: MEMORY.md
  exclude:
    - README.md

policy:
  types: [user, feedback, project, reference, decision]
  # Un fait par note. Au-dela, la note ne se perime plus proprement.
  max_body_words: 400
  # Exiger un proprietaire : indispensable qu'on est plusieurs, sinon
  # personne ne supprime jamais rien.
  require_owner: false
  staleness:
    review_after_days: 180
    max_stale_ratio: 0.15

verify:
  timeout_seconds: 30
`

func trimPositional(args []string) []string {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[1:]
	}
	return args
}

func splitList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}
