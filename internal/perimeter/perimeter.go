// Package perimeter charge et valide le registre des sources d'un perimetre.
//
// Le registre declare quelle brique concrete sert quel role du modele de
// readiness, et comment prouver que chacune repond encore. Deux invariants le
// gouvernent, et ils sont appliques structurellement plutot que par
// convention :
//
//   - une source est referencee et sondee, jamais recopiee. Le schema n'offre
//     aucun champ de cache ou de miroir : declarer une copie locale est une
//     erreur de schema, pas une mauvaise pratique.
//   - un credential n'est jamais une valeur, seulement une reference resolue
//     a l'execution.
package perimeter

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/UnPoilTefal/perimeter/schema"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/yaml.v3"
)

// File est le nom conventionnel du registre.
const File = "perimeter.yml"

// Roles enumere les roles du modele de readiness. Ils ne sont pas
// configurables : un perimetre qui n'en pourvoit pas un a une carence, et le
// registre doit la nommer plutot que la laisser passer.
var Roles = []string{
	"intention.spec",
	"intention.tickets",
	"contrainte.decisions",
	"contrainte.memoire",
	"etat.declare",
	"etat.reel",
}

// Adapters liste les types de brique reconnus.
var Adapters = []string{"github", "gitlab", "jira", "git", "files", "http", "shell"}

// Command est une commande en lecture seule attachee a une source.
type Command struct {
	Cmd          string `yaml:"cmd"`
	ExpectExit   *int   `yaml:"expect_exit"`
	ExpectStdout string `yaml:"expect_stdout"`
	Note         string `yaml:"note"`
}

// WantExit rend le code de sortie attendu, 0 par defaut.
func (c Command) WantExit() int {
	if c.ExpectExit == nil {
		return 0
	}
	return *c.ExpectExit
}

// CorpusRole designe le role dont la source porte le corpus de memoire.
const CorpusRole = "contrainte.memoire"

// CorpusPolicy est la politique appliquee aux notes d'un corpus. Elle vit dans
// la source qui porte ce corpus, et nulle part ailleurs : une equipe n'ecrit
// qu'un fichier.
type CorpusPolicy struct {
	Index                string   `yaml:"index"`
	Exclude              []string `yaml:"exclude"`
	Types                []string `yaml:"types"`
	MaxBodyWords         int      `yaml:"max_body_words"`
	RequireOwner         bool     `yaml:"require_owner"`
	VerifyTimeoutSeconds int      `yaml:"verify_timeout_seconds"`
	Staleness            struct {
		ReviewAfterDays int     `yaml:"review_after_days"`
		MaxStaleRatio   float64 `yaml:"max_stale_ratio"`
	} `yaml:"staleness"`
	Links struct {
		IgnorePrefixes []string `yaml:"ignore_prefixes"`
		ExternalRoots  []string `yaml:"external_roots"`
	} `yaml:"links"`
}

// Source est une brique concrete du perimetre.
type Source struct {
	Adapter     string        `yaml:"adapter"`
	Endpoint    string        `yaml:"endpoint"`
	Reliability string        `yaml:"reliability"`
	Credential  string        `yaml:"credential"`
	Probe       *Command      `yaml:"probe"`
	Query       *Command      `yaml:"query"`
	Corpus      *CorpusPolicy `yaml:"corpus"`
	Note        string        `yaml:"note"`
}

// Binding rattache un role a une source.
type Binding struct {
	Source string `yaml:"source"`
	Note   string `yaml:"note"`
}

// Registry est le registre charge.
type Registry struct {
	Path    string
	Version int                `yaml:"version"`
	RoleMap map[string]Binding `yaml:"roles"`
	Sources map[string]Source  `yaml:"sources"`
}

// Find remonte l'arborescence depuis dir a la recherche d'un registre, comme
// on cherche un depot git. Une commande peut ainsi etre lancee depuis
// n'importe quel sous-repertoire du perimetre.
func Find(dir string) (string, bool) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(abs, File)
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, true
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return "", false
		}
		abs = parent
	}
}

// CorpusSource rend la source qui porte le corpus de memoire, son chemin
// absolu, et sa politique. Le chemin est resolu relativement au registre, pour
// qu'une commande lancee d'ailleurs vise le bon repertoire.
func (r *Registry) CorpusSource() (name string, root string, policy *CorpusPolicy, err error) {
	b, ok := r.RoleMap[CorpusRole]
	if !ok || b.Source == "" {
		return "", "", nil, fmt.Errorf("%s ne pourvoit pas le role %q", r.Path, CorpusRole)
	}
	s, ok := r.Sources[b.Source]
	if !ok {
		return "", "", nil, fmt.Errorf("%s : le role %q renvoie a la source %q, absente", r.Path, CorpusRole, b.Source)
	}
	root = s.Endpoint
	if root == "" {
		root = "."
	}
	if !filepath.IsAbs(root) {
		root = filepath.Join(filepath.Dir(r.Path), root)
	}
	return b.Source, root, s.Corpus, nil
}

// Load lit un registre et le valide contre son schema.
func Load(path string) (*Registry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var generic map[string]any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("%s : YAML invalide : %w", path, err)
	}
	if err := validateSchema(generic); err != nil {
		return nil, fmt.Errorf("%s : %w", path, err)
	}
	r := &Registry{Path: path}
	if err := yaml.Unmarshal(raw, r); err != nil {
		return nil, fmt.Errorf("%s : %w", path, err)
	}
	return r, nil
}

var printer = message.NewPrinter(language.English)

func validateSchema(doc map[string]any) error {
	parsed, err := jsonschema.UnmarshalJSON(strings.NewReader(string(schema.Perimeter)))
	if err != nil {
		return err
	}
	cmp := jsonschema.NewCompiler()
	if err := cmp.AddResource("perimeter.schema.json", parsed); err != nil {
		return err
	}
	sch, err := cmp.Compile("perimeter.schema.json")
	if err != nil {
		return err
	}
	if err := sch.Validate(normalize(doc)); err != nil {
		ve, ok := err.(*jsonschema.ValidationError)
		if !ok {
			return err
		}
		var msgs []string
		var walk func(e *jsonschema.ValidationError)
		walk = func(e *jsonschema.ValidationError) {
			if len(e.Causes) == 0 {
				loc := "/"
				if len(e.InstanceLocation) > 0 {
					loc = "/" + strings.Join(e.InstanceLocation, "/")
				}
				msgs = append(msgs, fmt.Sprintf("%s : %s", loc, e.ErrorKind.LocalizedString(printer)))
				return
			}
			for _, c := range e.Causes {
				walk(c)
			}
		}
		walk(ve)
		if len(msgs) > 4 {
			msgs = append(msgs[:4], fmt.Sprintf("… et %d autres", len(msgs)-4))
		}
		return fmt.Errorf("registre non conforme :\n  - %s", strings.Join(msgs, "\n  - "))
	}
	return nil
}

func normalize(v any) any {
	switch t := v.(type) {
	case map[string]any:
		m := make(map[string]any, len(t))
		for k, vv := range t {
			m[k] = normalize(vv)
		}
		return m
	case map[any]any:
		m := make(map[string]any, len(t))
		for k, vv := range t {
			m[fmt.Sprint(k)] = normalize(vv)
		}
		return m
	case []any:
		s := make([]any, len(t))
		for i, vv := range t {
			s[i] = normalize(vv)
		}
		return s
	case int:
		return float64(t)
	default:
		return v
	}
}

// Issue est un constat de coherence du registre.
type Issue struct {
	Role    string
	Source  string
	Message string
	Hint    string
}

func (i Issue) String() string {
	subject := i.Role
	if subject == "" {
		subject = "source " + i.Source
	}
	s := fmt.Sprintf("%s : %s", subject, i.Message)
	if i.Hint != "" {
		s += "\n    " + i.Hint
	}
	return s
}

// literalSecret attrape une valeur de credential ecrite en clair la ou seule
// une reference est admise. Le schema impose deja un prefixe connu ; ceci
// couvre le cas du prefixe correct suivi de ce qui ressemble a un secret.
var literalSecret = regexp.MustCompile(`^(env|file|keychain|cmd|op):.*(gh[pousr]_[A-Za-z0-9]{20,}|glpat-[A-Za-z0-9_\-]{16,}|AKIA[0-9A-Z]{16})`)

// Check verifie la coherence du registre : couverture des roles, existence
// des sources referencees, adaptateurs connus, credentials en reference.
//
// C'est la condition E1 du demarrage : tant qu'un role n'est pas pourvu et
// sondable, le perimetre n'est pas decrit.
func (r *Registry) Check() []Issue {
	var issues []Issue

	for _, role := range Roles {
		b, ok := r.RoleMap[role]
		if !ok || strings.TrimSpace(b.Source) == "" {
			issues = append(issues, Issue{
				Role:    role,
				Message: "role non pourvu",
				Hint:    "declarer la brique qui le sert, ou expliciter qu'elle n'existe pas encore",
			})
			continue
		}
		if _, ok := r.Sources[b.Source]; !ok {
			issues = append(issues, Issue{
				Role:    role,
				Message: fmt.Sprintf("renvoie a la source %q, absente du registre", b.Source),
			})
		}
	}

	names := make([]string, 0, len(r.Sources))
	for name := range r.Sources {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		s := r.Sources[name]
		if !known(Adapters, s.Adapter) {
			issues = append(issues, Issue{
				Source:  name,
				Message: fmt.Sprintf("adaptateur inconnu : %q", s.Adapter),
				Hint:    "adaptateurs reconnus : " + strings.Join(Adapters, ", "),
			})
		}
		if s.Probe == nil || strings.TrimSpace(s.Probe.Cmd) == "" {
			issues = append(issues, Issue{
				Source:  name,
				Message: "aucune sonde declaree",
				Hint:    "une source sans sonde se degrade en silence : rien ne dira que l'acces est perdu",
			})
		}
		if s.Credential != "" && literalSecret.MatchString(s.Credential) {
			issues = append(issues, Issue{
				Source:  name,
				Message: "le credential ressemble a une valeur, pas a une reference",
				Hint:    "le registre ne porte que des references resolues a l'execution",
			})
		}
	}

	// Une source declaree que plus aucun role n'utilise est du poids mort.
	used := map[string]bool{}
	for _, b := range r.RoleMap {
		used[b.Source] = true
	}
	for _, name := range names {
		if !used[name] {
			issues = append(issues, Issue{
				Source:  name,
				Message: "declaree mais rattachee a aucun role",
				Hint:    "la rattacher, ou la retirer du registre",
			})
		}
	}
	return issues
}

func known(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// Resolve rend la commande a executer pour interroger une source, arg
// substitue. Une source sans interrogation parametree ne peut servir qu'a sa
// propre sonde.
func (r *Registry) Resolve(source, arg string) (Command, error) {
	s, ok := r.Sources[source]
	if !ok {
		return Command{}, fmt.Errorf("source %q absente du registre %s", source, r.Path)
	}
	if s.Query == nil || strings.TrimSpace(s.Query.Cmd) == "" {
		return Command{}, fmt.Errorf("source %q (adaptateur %s) ne declare aucune interrogation parametree", source, s.Adapter)
	}
	c := *s.Query
	c.Cmd = substitute(c.Cmd, s.Endpoint, arg)
	return c, nil
}

// ProbeOf rend la sonde d'une source, endpoint substitue.
func (r *Registry) ProbeOf(source string) (Command, error) {
	s, ok := r.Sources[source]
	if !ok {
		return Command{}, fmt.Errorf("source %q absente du registre %s", source, r.Path)
	}
	if s.Probe == nil {
		return Command{}, fmt.Errorf("source %q ne declare aucune sonde", source)
	}
	c := *s.Probe
	c.Cmd = substitute(c.Cmd, s.Endpoint, "")
	return c, nil
}

// SourceNames rend les identifiants de source, tries.
func (r *Registry) SourceNames() []string {
	names := make([]string, 0, len(r.Sources))
	for n := range r.Sources {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func substitute(cmd, endpoint, arg string) string {
	cmd = strings.ReplaceAll(cmd, "${endpoint}", endpoint)
	cmd = strings.ReplaceAll(cmd, "${arg}", arg)
	return cmd
}
