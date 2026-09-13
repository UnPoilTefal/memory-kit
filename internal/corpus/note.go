// Package corpus charge et represente un corpus de memoire : des notes
// markdown atomiques a frontmatter YAML, plus un index qui sert de routeur.
package corpus

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Check est une preuve attachee a un fait. Elle prend l'une de deux formes,
// exclusives : une commande ecrite dans la note, ou le renvoi a une source du
// registre de perimetre. La seconde est preferable des qu'un perimetre est
// declare — elle survit au changement d'outil, la premiere non.
type Check struct {
	Cmd          string `yaml:"cmd"`
	Source       string `yaml:"source"`
	Arg          string `yaml:"arg"`
	ExpectExit   *int   `yaml:"expect_exit"`
	ExpectStdout string `yaml:"expect_stdout"`
	Note         string `yaml:"note"`
}

// ViaSource dit si la preuve passe par le registre plutot que par une
// commande locale.
func (c Check) ViaSource() bool { return c.Source != "" }

// WantExit rend le code de sortie attendu, 0 par defaut.
func (c Check) WantExit() int {
	if c.ExpectExit == nil {
		return 0
	}
	return *c.ExpectExit
}

// Metadata est le bloc metadata du frontmatter. Les champs inconnus sont
// tolerees : un corpus existant ne doit pas casser a l'adoption de l'outil.
type Metadata struct {
	Type         string  `yaml:"type"`
	Trust        string  `yaml:"trust"`
	Owner        string  `yaml:"owner"`
	Source       string  `yaml:"source"`
	Modified     string  `yaml:"modified"`
	ReviewAfter  string  `yaml:"review_after"`
	Verify       []Check `yaml:"verify"`
	VerifiedAt   string  `yaml:"verified_at"`
	VerifyStatus string  `yaml:"verify_status"`
}

// Note est une note de memoire chargee depuis le disque.
type Note struct {
	Path     string // chemin absolu
	Rel      string // chemin relatif a la racine du corpus
	Base     string // nom de fichier
	Name     string `yaml:"name"`
	Desc     string `yaml:"description"`
	Metadata Metadata

	Body     string
	BodyWords int

	// Root est l'arbre YAML du frontmatter, conserve pour une reecriture
	// chirurgicale qui preserve ordre et commentaires.
	Root *yaml.Node
	// Any est le frontmatter en representation generique, pour la
	// validation contre le JSON Schema.
	Any map[string]any

	// ParseErr est non nil si le frontmatter est illisible. La note est
	// alors rapportee mais pas analysee plus loin.
	ParseErr error
}

var fenceSep = []byte("---")

// splitFrontmatter separe le bloc YAML en tete du corps markdown.
func splitFrontmatter(raw []byte) (fm, body []byte, err error) {
	trimmed := bytes.TrimLeft(raw, "\ufeff \t\r\n")
	if !bytes.HasPrefix(trimmed, fenceSep) {
		return nil, nil, fmt.Errorf("frontmatter absent : le fichier doit commencer par ---")
	}
	rest := trimmed[len(fenceSep):]
	rest = bytes.TrimLeft(rest, " \t\r")
	if !bytes.HasPrefix(rest, []byte("\n")) {
		return nil, nil, fmt.Errorf("frontmatter absent : --- doit etre seul sur sa ligne")
	}
	rest = rest[1:]

	// Cherche la cloture : une ligne ne contenant que ---
	lines := bytes.Split(rest, []byte("\n"))
	for i, ln := range lines {
		if bytes.Equal(bytes.TrimRight(ln, " \t\r"), fenceSep) {
			fm = bytes.Join(lines[:i], []byte("\n"))
			body = bytes.Join(lines[i+1:], []byte("\n"))
			return fm, body, nil
		}
	}
	return nil, nil, fmt.Errorf("frontmatter non clos : --- de fermeture manquant")
}

// LoadNote lit et analyse une note. Une erreur de parsing est portee par la
// note elle-meme plutot que remontee, pour que le lint puisse rapporter
// toutes les notes cassees d'un coup au lieu de s'arreter a la premiere.
func LoadNote(root, path string) (*Note, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	rel, _ := filepath.Rel(root, path)
	n := &Note{Path: path, Rel: rel, Base: filepath.Base(path)}

	fm, body, err := splitFrontmatter(raw)
	if err != nil {
		n.ParseErr = err
		return n, nil
	}
	n.Body = strings.TrimSpace(string(body))
	n.BodyWords = len(strings.Fields(n.Body))

	var doc yaml.Node
	if err := yaml.Unmarshal(fm, &doc); err != nil {
		n.ParseErr = fmt.Errorf("YAML invalide : %w", err)
		return n, nil
	}
	if len(doc.Content) == 0 {
		n.ParseErr = fmt.Errorf("frontmatter vide")
		return n, nil
	}
	n.Root = doc.Content[0]

	var shape struct {
		Name string   `yaml:"name"`
		Desc string   `yaml:"description"`
		Meta Metadata `yaml:"metadata"`
	}
	if err := yaml.Unmarshal(fm, &shape); err != nil {
		n.ParseErr = fmt.Errorf("frontmatter non conforme : %w", err)
		return n, nil
	}
	n.Name, n.Desc, n.Metadata = shape.Name, shape.Desc, shape.Meta

	if err := yaml.Unmarshal(fm, &n.Any); err != nil {
		n.ParseErr = fmt.Errorf("frontmatter non convertible : %w", err)
		return n, nil
	}
	return n, nil
}

// ReviewDue rend la date a laquelle la note doit etre relue, et si elle est
// connue. review_after explicite gagne ; sinon on part de la derniere preuve
// (verified_at) ou de la derniere modification, plus le budget du corpus.
func (n *Note) ReviewDue(budgetDays int) (time.Time, bool) {
	if n.Metadata.ReviewAfter != "" {
		if t, err := time.Parse("2006-01-02", n.Metadata.ReviewAfter); err == nil {
			return t, true
		}
	}
	if budgetDays <= 0 {
		return time.Time{}, false
	}
	base, ok := n.anchorDate()
	if !ok {
		return time.Time{}, false
	}
	return base.AddDate(0, 0, budgetDays), true
}

// anchorDate est la date la plus recente prouvant que quelqu'un a regarde la note.
func (n *Note) anchorDate() (time.Time, bool) {
	var best time.Time
	for _, raw := range []string{n.Metadata.VerifiedAt, n.Metadata.Modified} {
		if t, ok := parseLooseDate(raw); ok && t.After(best) {
			best = t
		}
	}
	if best.IsZero() {
		return time.Time{}, false
	}
	return best, true
}

func parseLooseDate(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.000Z", "2006-01-02", "2006-01-02 15:04:05"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	if len(s) >= 10 {
		if t, err := time.Parse("2006-01-02", s[:10]); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// IndexHook rend l'accroche que l'index doit porter pour cette note : la
// premiere proposition de la description. C'est la description qui declenche
// le rappel, donc c'est elle, et pas une reformulation, qui doit figurer au
// routeur — sinon l'index derive de la note qu'il route, en silence.
func (n *Note) IndexHook() string {
	d := strings.TrimSpace(n.Desc)
	for _, sep := range []string{" — ", " ; ", ". "} {
		if i := strings.Index(d, sep); i > 0 {
			d = d[:i]
			break
		}
	}
	return strings.TrimSpace(strings.Trim(d, "\""))
}

// IndexTitle derive un intitule lisible du nom de fichier.
func (n *Note) IndexTitle() string {
	slug := strings.TrimSuffix(n.Base, ".md")
	parts := strings.SplitN(slug, "-", 2)
	if len(parts) == 2 && parts[0] != "" {
		kind := strings.ToUpper(parts[0][:1]) + parts[0][1:]
		return kind + ": " + strings.ReplaceAll(parts[1], "-", " ")
	}
	return strings.ReplaceAll(slug, "-", " ")
}

// IndexLine rend la ligne d'index canonique de la note.
func (n *Note) IndexLine(title string) string {
	if title == "" {
		title = n.IndexTitle()
	}
	return "- [" + title + "](" + n.Rel + ") — " + n.IndexHook()
}
