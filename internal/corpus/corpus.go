package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// ConfigFile est le nom du fichier de configuration a la racine d'un corpus.
const ConfigFile = ".corpus.yml"

// Config decrit un corpus. Les valeurs par defaut correspondent a la
// disposition d'un repertoire de memoire Claude Code (notes a plat, index
// MEMORY.md), pour qu'un corpus existant marche sans configuration.
type Config struct {
	Version int `yaml:"version"`

	Corpus struct {
		Path    string   `yaml:"path"`
		Index   string   `yaml:"index"`
		Exclude []string `yaml:"exclude"`
	} `yaml:"corpus"`

	Policy struct {
		Types       []string `yaml:"types"`
		MaxBodyWords int     `yaml:"max_body_words"`
		Staleness   struct {
			ReviewAfterDays int     `yaml:"review_after_days"`
			MaxStaleRatio   float64 `yaml:"max_stale_ratio"`
		} `yaml:"staleness"`
		RequireOwner bool `yaml:"require_owner"`
	} `yaml:"policy"`

	// Links declare les corpus voisins vers lesquels un lien est legitime.
	// Un corpus de memoire n'est jamais seul : il cite un wiki, des
	// runbooks, des commandes. Sans cette declaration, l'outil ne peut pas
	// distinguer un lien externe valide d'un lien casse.
	Links struct {
		IgnorePrefixes []string `yaml:"ignore_prefixes"`
		ExternalRoots  []string `yaml:"external_roots"`
	} `yaml:"links"`

	Verify struct {
		TimeoutSeconds int `yaml:"timeout_seconds"`
	} `yaml:"verify"`
}

// DefaultConfig rend la configuration appliquee en l'absence de .corpus.yml.
func DefaultConfig() *Config {
	c := &Config{Version: 1}
	c.Corpus.Path = "."
	c.Corpus.Index = "MEMORY.md"
	c.Policy.Types = []string{"user", "feedback", "project", "reference", "decision"}
	c.Policy.MaxBodyWords = 400
	c.Policy.Staleness.ReviewAfterDays = 180
	c.Policy.Staleness.MaxStaleRatio = 0.15
	// Un lien commencant par / designe une commande ou une skill, pas une note.
	c.Links.IgnorePrefixes = []string{"/"}
	c.Verify.TimeoutSeconds = 30
	return c
}

// Corpus est un ensemble de notes charge depuis le disque.
type Corpus struct {
	Root      string
	Config    *Config
	Notes     []*Note
	IndexPath string
	// IndexTargets liste les fichiers cites par l'index, dans l'ordre.
	IndexTargets []string
	// IndexEntries decrit les entrees completes reconnues par l'outil.
	IndexEntries []IndexEntry
	HasIndex     bool
}

// IndexEntry est une ligne d'index de la forme "- [titre](fichier.md) — accroche".
// L'accroche est de l'etat derive : elle doit reproduire la description de la
// note, faute de quoi le routeur finit par contredire ce qu'il route.
type IndexEntry struct {
	Line   int
	Raw    string
	Title  string
	Target string
	Hook   string
}

// LoadConfig lit .corpus.yml a la racine donnee, ou rend les defauts.
func LoadConfig(root string) (*Config, error) {
	cfg := DefaultConfig()
	raw, err := os.ReadFile(filepath.Join(root, ConfigFile))
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	if err := yaml.Unmarshal(raw, cfg); err != nil {
		return nil, fmt.Errorf("%s : %w", ConfigFile, err)
	}
	// Un champ absent du fichier retombe sur le defaut.
	d := DefaultConfig()
	if cfg.Corpus.Path == "" {
		cfg.Corpus.Path = d.Corpus.Path
	}
	if len(cfg.Policy.Types) == 0 {
		cfg.Policy.Types = d.Policy.Types
	}
	if cfg.Policy.MaxBodyWords == 0 {
		cfg.Policy.MaxBodyWords = d.Policy.MaxBodyWords
	}
	if cfg.Policy.Staleness.ReviewAfterDays == 0 {
		cfg.Policy.Staleness.ReviewAfterDays = d.Policy.Staleness.ReviewAfterDays
	}
	if cfg.Policy.Staleness.MaxStaleRatio == 0 {
		cfg.Policy.Staleness.MaxStaleRatio = d.Policy.Staleness.MaxStaleRatio
	}
	if cfg.Links.IgnorePrefixes == nil {
		cfg.Links.IgnorePrefixes = d.Links.IgnorePrefixes
	}
	if cfg.Verify.TimeoutSeconds == 0 {
		cfg.Verify.TimeoutSeconds = d.Verify.TimeoutSeconds
	}
	return cfg, nil
}

// indexLink capture les cibles markdown d'un lien : [titre](fichier.md)
var indexLink = regexp.MustCompile(`\]\(([^)\s]+\.md)\)`)

// indexEntry reconnait une entree complete : puce, titre, cible, accroche.
var indexEntry = regexp.MustCompile(`^\s*[-*]\s*\[([^\]]*)\]\(([^)\s]+\.md)\)\s*(?:[—-]\s*(.*))?$`)

// Load charge le corpus enracine en root.
func Load(root string) (*Corpus, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	cfg, err := LoadConfig(abs)
	if err != nil {
		return nil, err
	}
	notesRoot := filepath.Join(abs, cfg.Corpus.Path)
	c := &Corpus{Root: notesRoot, Config: cfg}

	if cfg.Corpus.Index != "" {
		c.IndexPath = filepath.Join(notesRoot, cfg.Corpus.Index)
		if raw, err := os.ReadFile(c.IndexPath); err == nil {
			c.HasIndex = true
			for _, m := range indexLink.FindAllStringSubmatch(string(raw), -1) {
				c.IndexTargets = append(c.IndexTargets, m[1])
			}
			for i, line := range strings.Split(string(raw), "\n") {
				if m := indexEntry.FindStringSubmatch(line); m != nil {
					c.IndexEntries = append(c.IndexEntries, IndexEntry{
						Line: i + 1, Raw: line, Title: m[1], Target: m[2],
						Hook: strings.TrimSpace(m[3]),
					})
				}
			}
		}
	}

	excluded := func(rel string) bool {
		if cfg.Corpus.Index != "" && rel == cfg.Corpus.Index {
			return true
		}
		for _, pat := range cfg.Corpus.Exclude {
			if ok, _ := filepath.Match(pat, rel); ok {
				return true
			}
			if ok, _ := filepath.Match(pat, filepath.Base(rel)); ok {
				return true
			}
		}
		return false
	}

	err = filepath.WalkDir(notesRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != notesRoot && (strings.HasPrefix(name, ".") || name == "node_modules") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		rel, _ := filepath.Rel(notesRoot, path)
		if excluded(rel) {
			return nil
		}
		n, err := LoadNote(notesRoot, path)
		if err != nil {
			return err
		}
		c.Notes = append(c.Notes, n)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(c.Notes, func(i, j int) bool { return c.Notes[i].Rel < c.Notes[j].Rel })
	return c, nil
}

// ExternalStems rend l'ensemble des titres de notes atteignables dans les
// corpus voisins declares, indexes comme le fait un vault : par nom de
// fichier sans extension, quel que soit le sous-repertoire.
func (c *Corpus) ExternalStems() map[string]string {
	stems := map[string]string{}
	for _, root := range c.Config.Links.ExternalRoots {
		root = expandHome(root)
		if !filepath.IsAbs(root) {
			root = filepath.Join(c.Root, root)
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if path != root && strings.HasPrefix(d.Name(), ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(d.Name(), ".md") {
				stems[strings.TrimSuffix(d.Name(), ".md")] = path
			}
			return nil
		})
	}
	return stems
}

func expandHome(p string) string {
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, p[2:])
		}
	}
	return p
}

// ByRel indexe les notes par chemin relatif.
func (c *Corpus) ByRel() map[string]*Note {
	m := make(map[string]*Note, len(c.Notes))
	for _, n := range c.Notes {
		m[n.Rel] = n
	}
	return m
}

// ByName indexe les notes par champ name.
func (c *Corpus) ByName() map[string]*Note {
	m := make(map[string]*Note, len(c.Notes))
	for _, n := range c.Notes {
		if n.Name != "" {
			m[n.Name] = n
		}
	}
	return m
}
