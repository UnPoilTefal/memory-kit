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
	"time"

	"github.com/UnPoilTefal/memory-kit/internal/corpus"
	"github.com/UnPoilTefal/memory-kit/internal/lint"
	"github.com/UnPoilTefal/memory-kit/internal/perimeter"
	"github.com/UnPoilTefal/memory-kit/internal/readiness"
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
  memctl gate <evaluation> derive le verdict de readiness d'une specification
  memctl readiness         etat de sortie de demarrage, et regime qui en decoule
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
	case "gate":
		err = cmdGate(os.Args[2:])
	case "readiness":
		err = cmdReadiness(os.Args[2:])
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

// cmdGate derive le verdict d'une evaluation de readiness et consigne le
// passage. Le code de sortie porte le verdict : 0 produire, 1 instruire,
// 2 rendre la main — pour qu'une automatisation puisse s'y brancher.
func cmdGate(args []string) error {
	fs := flag.NewFlagSet("gate", flag.ExitOnError)
	regPath := fs.String("perimeter", "", "registre des sources, pour verifier la couverture des roles")
	corpusPath := fs.String("corpus", "", "corpus de memoire, pour verifier la fraicheur des faits mobilises")
	ledger := fs.String("ledger", readiness.LedgerFile, "journal des passages")
	noRecord := fs.Bool("no-record", false, "ne consigne pas ce passage")
	path := target(args)
	_ = fs.Parse(trimPositional(args))
	if path == "." {
		return fmt.Errorf("indiquer le fichier d'evaluation a passer")
	}

	a, err := readiness.Load(path)
	if err != nil {
		return err
	}
	if issues := a.Coherence(); len(issues) > 0 {
		fmt.Fprintln(os.Stderr, "evaluation incoherente :") //nolint:errcheck // sortie terminal
		for _, i := range issues {
			fmt.Fprintf(os.Stderr, "  - %s\n", i) //nolint:errcheck // sortie terminal
		}
		return fmt.Errorf("%d incoherence(s) : une carence omise rendrait le verdict faussement vert", len(issues))
	}

	pre, err := preconditions(*regPath, *corpusPath, a)
	if err != nil {
		return err
	}
	d := a.Derive(pre)

	fmt.Printf("%s\n\n  verdict : %s\n", d.Spec, strings.ToUpper(string(d.Verdict)))
	for _, r := range d.Reasons {
		fmt.Printf("  · %s\n", r)
	}
	if d.Escalation != "" {
		fmt.Printf("\n  escalade classee : %s\n", d.Escalation)
	}

	if !*noRecord {
		e := readiness.Entry{
			At: time.Now().UTC(), Spec: a.Spec, Verdict: d.Verdict,
			Escalation: d.Escalation, Assessment: filepath.Base(path),
		}
		if err := readiness.Append(*ledger, e); err != nil {
			return fmt.Errorf("journal : %w", err)
		}
		fmt.Printf("\n  consigne dans %s\n", *ledger)
	}

	switch d.Verdict {
	case readiness.Produire:
		return nil
	case readiness.Instruire:
		return fail(1)
	default:
		return fail(2)
	}
}

// preconditions rassemble ce que l'outil sait verifier seul : la couverture
// du perimetre, et la fraicheur des faits sur lesquels l'evaluation s'appuie.
// S'appuyer sur un fait dont la preuve ne tient plus, c'est agir sur une
// premisse fausse — exactement ce que la porte existe pour empecher.
func preconditions(regPath, corpusPath string, a *readiness.Assessment) ([]readiness.Precondition, error) {
	var pre []readiness.Precondition

	if regPath != "" {
		reg, err := perimeter.Load(regPath)
		if err != nil {
			return nil, err
		}
		if issues := reg.Check(); len(issues) > 0 {
			pre = append(pre, readiness.Precondition{
				Code:    "perimetre",
				Message: fmt.Sprintf("%d constat(s) sur le registre : le perimetre n'est pas entierement decrit", len(issues)),
				Hint:    "memctl perimeter " + regPath,
			})
		}
	}

	if corpusPath == "" || len(a.Facts) == 0 {
		return pre, nil
	}
	c, err := corpus.Load(corpusPath)
	if err != nil {
		return nil, err
	}
	byName := c.ByName()
	now := time.Now()
	for _, name := range a.Facts {
		n, ok := byName[name]
		if !ok {
			pre = append(pre, readiness.Precondition{
				Code:    "fait-absent",
				Message: fmt.Sprintf("le fait %q n'existe pas dans le corpus", name),
			})
			continue
		}
		if n.Metadata.VerifyStatus == "fail" {
			pre = append(pre, readiness.Precondition{
				Code:    "preuve-en-echec",
				Message: fmt.Sprintf("le fait %q porte une preuve en echec", name),
				Hint:    "relire le fait avant de s'y appuyer",
			})
			continue
		}
		if due, ok := n.ReviewDue(c.Config.Policy.Staleness.ReviewAfterDays); ok && now.After(due) {
			pre = append(pre, readiness.Precondition{
				Code:    "fait-perime",
				Message: fmt.Sprintf("le fait %q est a relire depuis le %s", name, due.Format("2006-01-02")),
			})
		}
	}
	return pre, nil
}

// cmdReadiness rend l'etat de sortie de demarrage. Le regime n'est pas un
// reglage : il se deduit du journal, donc il ne peut pas pourrir.
func cmdReadiness(args []string) error {
	fs := flag.NewFlagSet("readiness", flag.ExitOnError)
	ledger := fs.String("ledger", readiness.LedgerFile, "journal des passages")
	window := fs.Int("window", readiness.DefaultWindow, "nombre de passages consecutifs juges")
	_ = fs.Parse(trimPositional(args))

	entries, err := readiness.ReadLedger(*ledger)
	if err != nil {
		return err
	}
	m := readiness.Assess(entries, *window)

	phase := "demarrage"
	if !m.Bootstrap {
		phase = "regime etabli"
	}
	fmt.Printf("%s — %d passage(s) consignes\n\n", *ledger, len(entries))
	fmt.Printf("  phase  : %s\n", phase)
	fmt.Printf("  regime : %s\n", m.Mode)
	fmt.Printf("  %s\n", m.Reason)

	if len(entries) > 0 {
		fmt.Printf("\n  derniers passages :\n")
		from := len(entries) - m.Window
		if from < 0 {
			from = 0
		}
		for _, e := range entries[from:] {
			esc := ""
			if e.Escalation != "" {
				esc = " (" + e.Escalation + ")"
			}
			fmt.Printf("    %s  %-14s%s  %s\n", e.At.Format("2006-01-02"), e.Verdict, esc, e.Spec)
		}
	}
	return nil
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
