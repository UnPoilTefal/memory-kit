// Package draft verifie un brouillon de note avant qu'il ne soit ecrit.
//
// C'est le portillon d'ecriture cote outil. Il ne repond pas aux trois
// questions du portillon — non re-derivable, non ephemere, comptera dans
// trois mois : ce sont des jugements, et pretendre les mecaniser donnerait
// une fausse assurance. Il fait ce qui est mecanique, et il n'ecrit jamais.
package draft

import (
	"regexp"
	"sort"
	"strings"

	"github.com/UnPoilTefal/perimeter/internal/claim"
	"github.com/UnPoilTefal/perimeter/internal/corpus"
	"github.com/UnPoilTefal/perimeter/internal/lint"
	"github.com/UnPoilTefal/perimeter/internal/perimeter"
	"github.com/UnPoilTefal/perimeter/internal/report"
)

// reglesDuCorpus ne s'appliquent pas a une note qui n'est pas encore ecrite :
// un brouillon n'est evidemment pas dans l'index, et sa date de relecture
// n'est pas encore passee. Les signaler serait produire des constats que rien
// ne permet de corriger.
var reglesDuCorpus = []string{
	"index-orphan", "index-dangling", "index-drift",
	"staleness", "staleness-budget",
}

// Options regle le volume du voisinage rendu.
type Options struct {
	// Voisins est le nombre de notes proches rendues. Zero prend le defaut.
	//
	// C'est un plafond, pas un seuil : la mesure montre que le signal
	// classe correctement mais que son echelle absolue ne veut rien dire
	// — le meilleur voisin median d'un corpus entretenu vaut 0,106, du
	// bruit. Seuiller rendrait un verdict que la mesure ne soutient pas ;
	// plafonner rend un cout de relecture connu d'avance.
	Voisins int
}

const voisinsParDefaut = 3

// Voisin est une note existante proche du brouillon, avec son motif.
type Voisin struct {
	Note   *corpus.Note
	Score  float64
	Termes []string
}

// Verdict est ce que l'outil sait dire d'un brouillon. Rien n'y est ecrit.
type Verdict struct {
	// Findings sont les constats de lint qui portent sur la note elle-meme.
	Findings []report.Finding
	// Collision est la note existante qui porte deja ce nom, s'il y en a une.
	Collision *corpus.Note
	// Voisins repond a « est-ce que ca existe deja ? ». Ordonne, jamais
	// conclusif : c'est a l'agent de juger.
	Voisins []Voisin
	// Verify est le bloc de preuve propose, quand la note enonce un fait
	// testable.
	Verify *claim.Proposal
}

// Bloquant dit si un constat interdit d'ecrire en l'etat.
func (v *Verdict) Bloquant() bool {
	if v.Collision != nil {
		return true
	}
	for _, f := range v.Findings {
		if f.Severity == report.Error {
			return true
		}
	}
	return false
}

// Check juge un brouillon contre un corpus existant. Le corpus n'est jamais
// modifie, ni en memoire ni sur disque : le brouillon est ajoute a une copie
// de la liste de notes, le temps de faire tourner les regles.
func Check(c *corpus.Corpus, n *corpus.Note, reg *perimeter.Registry, opts Options) (*Verdict, error) {
	v := &Verdict{}

	for _, existante := range c.Notes {
		if existante.Rel == n.Rel || (n.Name != "" && existante.Name == n.Name) {
			v.Collision = existante
			break
		}
	}

	augmente := avecBrouillon(c, n)

	res, err := lint.Run(augmente, lint.Options{Disable: reglesDuCorpus})
	if err != nil {
		return nil, err
	}
	for _, f := range res.Findings {
		if f.File == n.Rel {
			v.Findings = append(v.Findings, f)
		}
	}

	v.Voisins = voisinage(c, n, opts.Voisins)

	for _, p := range claim.Propose(augmente, reg).Proposals {
		if p.Note == n.Rel {
			prop := p
			v.Verify = &prop
			break
		}
	}
	return v, nil
}

// avecBrouillon rend un corpus dont la liste de notes contient le brouillon.
// La structure est copiee, pas mutee : le corpus d'origine sert encore au
// voisinage, et un appelant qui le reutilise ne doit rien voir changer.
func avecBrouillon(c *corpus.Corpus, n *corpus.Note) *corpus.Corpus {
	notes := make([]*corpus.Note, 0, len(c.Notes)+1)
	for _, existante := range c.Notes {
		if existante.Rel == n.Rel {
			continue
		}
		notes = append(notes, existante)
	}
	copie := *c
	copie.Notes = append(notes, n)
	return &copie
}

var motRe = regexp.MustCompile(`[^\p{L}\p{N}]+`)

// vides sont les mots trop frequents pour porter du sens. La liste est
// volontairement courte : un mot rare de trop coute une paire mal classee,
// un mot utile retire coute un voisin manque.
var vides = map[string]bool{}

func init() {
	for _, m := range strings.Fields(`les des une est sont pas plus que qui dans pour par sur avec sans cette cet son ses leur leurs tout tous toute toutes mais donc car aussi meme deja encore chaque entre vers depuis quand comme etre fait plutot hors sous toujours jamais faut doit avant apres autre`) {
		vides[m] = true
	}
}

// termes rend les mots porteurs de la description et du nom. C'est la
// description qui est comparee, pas le corps : c'est elle que l'agent lit
// pour decider si la note est pertinente.
func termes(n *corpus.Note) map[string]bool {
	brut := n.Desc + " " + strings.ReplaceAll(strings.TrimSuffix(n.Base, ".md"), "-", " ")
	out := map[string]bool{}
	for _, m := range motRe.Split(replierAccents(strings.ToLower(brut)), -1) {
		if len([]rune(m)) > 2 && !vides[m] {
			out[m] = true
		}
	}
	return out
}

// Voisinage rend les notes du corpus les plus proches d'une note donnee, avec
// leurs termes communs. Exporte parce que « harvest » pose exactement la meme
// question qu'un brouillon humain — « est-ce que ca existe deja ? » — et n'a
// aucune raison de la reimplementer.
func Voisinage(c *corpus.Corpus, n *corpus.Note, plafond int) []Voisin {
	return voisinage(c, n, plafond)
}

func voisinage(c *corpus.Corpus, n *corpus.Note, plafond int) []Voisin {
	if plafond <= 0 {
		plafond = voisinsParDefaut
	}
	tn := termes(n)
	if len(tn) == 0 {
		return nil
	}
	var out []Voisin
	for _, existante := range c.Notes {
		if existante.Rel == n.Rel || existante.ParseErr != nil {
			continue
		}
		te := termes(existante)
		if len(te) == 0 {
			continue
		}
		var communs []string
		for m := range tn {
			if te[m] {
				communs = append(communs, m)
			}
		}
		if len(communs) == 0 {
			continue
		}
		union := len(tn) + len(te) - len(communs)
		sort.Strings(communs)
		out = append(out, Voisin{Note: existante, Score: float64(len(communs)) / float64(union), Termes: communs})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].Note.Rel < out[j].Note.Rel
	})
	if len(out) > plafond {
		out = out[:plafond]
	}
	return out
}

// replierAccents ramene les diacritiques latines a leur lettre de base.
// Mesure sur un corpus francais reel : sans ce repli, « verifier » dans un
// brouillon ne rencontre jamais « verifier » accentue dans les notes, et le
// voisinage passe a cote de la note qu'il devait justement remonter.
func replierAccents(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if repli, ok := accents[r]; ok {
			b.WriteRune(repli)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

var accents = map[rune]rune{
	'à': 'a', 'â': 'a', 'ä': 'a', 'á': 'a', 'ã': 'a', 'å': 'a',
	'é': 'e', 'è': 'e', 'ê': 'e', 'ë': 'e',
	'î': 'i', 'ï': 'i', 'í': 'i', 'ì': 'i',
	'ô': 'o', 'ö': 'o', 'ó': 'o', 'ò': 'o', 'õ': 'o',
	'ù': 'u', 'û': 'u', 'ü': 'u', 'ú': 'u',
	'ç': 'c', 'ñ': 'n', 'ÿ': 'y',
}
