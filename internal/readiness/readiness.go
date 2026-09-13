// Package readiness porte la porte de readiness et son journal.
//
// Partage du travail assume : l'outil ne juge pas si une specification est
// comprise — il ne sait pas lire. Il fournit le cadre et le registre des
// passages ; l'agent fournit le jugement. C'est la meme division que pour les
// preuves : quelqu'un ecrit la sonde, l'outil la rejoue.
//
// Le verdict n'est donc jamais declare par l'auteur. Il est derive de ce que
// l'auteur a consigne — tests ecrits ou bloques, carences classees, faits
// mobilises — et des preconditions mecaniques que l'outil sait verifier.
package readiness

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/UnPoilTefal/memory-kit/schema"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gopkg.in/yaml.v3"
)

// Verdict est l'une des trois issues de la porte.
type Verdict string

const (
	// Produire : les tests sont ecrits, rien ne bloque.
	Produire Verdict = "produire"
	// Instruire : la carence est mesurable, l'agent va la combler seul.
	Instruire Verdict = "instruire"
	// RendreLaMain : la carence est irreductible par l'agent.
	RendreLaMain Verdict = "rendre-la-main"
)

// Classifications de carence. La classification decide de l'issue ; c'est
// pour cela qu'elle est un champ ferme et non un commentaire.
const (
	Mesurable    = "mesurable"
	Connaissance = "connaissance"
	Intention    = "intention"
)

// Test est une assertion d'acceptation, ecrite ou empechee.
type Test struct {
	ID        string `yaml:"id"`
	Assertion string `yaml:"assertion"`
	Status    string `yaml:"status"`
	BlockedBy string `yaml:"blocked_by"`
}

// Deficiency est une carence rencontree en tentant d'ecrire les tests.
type Deficiency struct {
	ID             string `yaml:"id"`
	Classification string `yaml:"classification"`
	Statement      string `yaml:"statement"`
	Resolved       bool   `yaml:"resolved"`
	ResolvedBy     string `yaml:"resolved_by"`
}

// Assessment est une evaluation de readiness.
type Assessment struct {
	Path         string
	Version      int          `yaml:"version"`
	Spec         string       `yaml:"spec"`
	Tests        []Test       `yaml:"tests"`
	Deficiencies []Deficiency `yaml:"deficiencies"`
	Facts        []string     `yaml:"facts"`
}

var printer = message.NewPrinter(language.English)

// Load lit une evaluation et la valide contre son schema.
func Load(path string) (*Assessment, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var generic map[string]any
	if err := yaml.Unmarshal(raw, &generic); err != nil {
		return nil, fmt.Errorf("%s : YAML invalide : %w", path, err)
	}
	if err := validate(generic); err != nil {
		return nil, fmt.Errorf("%s : %w", path, err)
	}
	a := &Assessment{Path: path}
	if err := yaml.Unmarshal(raw, a); err != nil {
		return nil, fmt.Errorf("%s : %w", path, err)
	}
	return a, nil
}

func validate(doc map[string]any) error {
	parsed, err := jsonschema.UnmarshalJSON(strings.NewReader(string(schema.Readiness)))
	if err != nil {
		return err
	}
	cmp := jsonschema.NewCompiler()
	if err := cmp.AddResource("readiness.schema.json", parsed); err != nil {
		return err
	}
	sch, err := cmp.Compile("readiness.schema.json")
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
		return fmt.Errorf("evaluation non conforme :\n  - %s", strings.Join(msgs, "\n  - "))
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

// Precondition est un blocage mecanique constate par l'outil, independamment
// de ce que l'auteur a consigne.
type Precondition struct {
	Code    string
	Message string
	Hint    string
}

// Decision est le resultat d'un passage de porte.
type Decision struct {
	Spec          string
	Verdict       Verdict
	Reasons       []string
	Preconditions []Precondition
	// Escalation porte la classification qui a fait rendre la main, quand
	// c'est le cas. C'est elle qui alimente E3.
	Escalation string
}

// Derive rend le verdict. Les preconditions mecaniques sont fournies par
// l'appelant — l'outil sait verifier la couverture du perimetre et la
// fraicheur des faits, pas la comprehension d'une intention.
func (a *Assessment) Derive(pre []Precondition) Decision {
	d := Decision{Spec: a.Spec, Preconditions: pre}

	byClass := map[string][]Deficiency{}
	for _, def := range a.Deficiencies {
		byClass[def.Classification] = append(byClass[def.Classification], def)
	}

	// Une ambiguite d'intention est le seul arret legitime en regime etabli,
	// et elle prime sur tout le reste : aller mesurer ne sert a rien tant
	// qu'on ne sait pas ce qui est demande.
	if n := len(byClass[Intention]); n > 0 {
		d.Verdict = RendreLaMain
		d.Escalation = Intention
		d.Reasons = append(d.Reasons, fmt.Sprintf("%d carence(s) d'intention : reformuler avant d'instruire", n))
		return d
	}

	// Un savoir absent de toute source declaree n'est pas comblable par
	// l'agent : c'est une carence de couverture du perimetre.
	if n := len(byClass[Connaissance]); n > 0 {
		d.Verdict = RendreLaMain
		d.Escalation = Connaissance
		d.Reasons = append(d.Reasons, fmt.Sprintf("%d carence(s) de connaissance : les sources declarees ne couvrent pas le sujet", n))
		return d
	}

	var pending int
	for _, def := range byClass[Mesurable] {
		if !def.Resolved {
			pending++
		}
	}
	if pending > 0 {
		d.Verdict = Instruire
		d.Reasons = append(d.Reasons, fmt.Sprintf("%d carence(s) mesurable(s) non comblee(s)", pending))
		return d
	}

	// Une precondition mecanique ne fait jamais rendre la main : elle est par
	// construction quelque chose que l'agent peut aller regler.
	if len(pre) > 0 {
		d.Verdict = Instruire
		for _, p := range pre {
			d.Reasons = append(d.Reasons, p.Message)
		}
		return d
	}

	var blocked []string
	for _, t := range a.Tests {
		if t.Status == "blocked" {
			blocked = append(blocked, t.ID)
		}
	}
	if len(blocked) > 0 {
		sort.Strings(blocked)
		d.Verdict = Instruire
		d.Reasons = append(d.Reasons, fmt.Sprintf("tests non ecrits sans carence qui les explique : %s", strings.Join(blocked, ", ")))
		return d
	}

	d.Verdict = Produire
	d.Reasons = append(d.Reasons, fmt.Sprintf("%d test(s) d'acceptation ecrits, aucune carence ouverte", len(a.Tests)))
	return d
}

// Coherence verifie que l'evaluation se tient : un test bloque doit designer
// une carence existante, et une carence doit bloquer quelque chose ou etre
// resolue. Sans ca, on peut declarer un verdict vert en omettant de classer
// ce qui bloque.
func (a *Assessment) Coherence() []string {
	var issues []string
	known := map[string]bool{}
	for _, d := range a.Deficiencies {
		known[d.ID] = true
	}
	referenced := map[string]bool{}
	for _, t := range a.Tests {
		if t.Status != "blocked" {
			continue
		}
		if t.BlockedBy == "" {
			issues = append(issues, fmt.Sprintf("test %q est bloque sans designer de carence", t.ID))
			continue
		}
		if !known[t.BlockedBy] {
			issues = append(issues, fmt.Sprintf("test %q renvoie a la carence %q, absente", t.ID, t.BlockedBy))
		}
		referenced[t.BlockedBy] = true
	}
	for _, d := range a.Deficiencies {
		if d.Classification == Mesurable && d.Resolved {
			continue
		}
		if !referenced[d.ID] {
			issues = append(issues, fmt.Sprintf("carence %q ne bloque aucun test et n'est pas resolue", d.ID))
		}
	}
	return issues
}
