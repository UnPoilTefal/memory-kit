// Package harvest amorce un corpus depuis les traces qu'une equipe possede
// deja : historique git aujourd'hui, autres sources ensuite.
//
// Une equipe qui adopte l'outil part de zero, la ou un corpus organique met
// deux ans a se constituer. Harvest ne comble pas ces deux ans — il propose
// des candidats a relire, et rien d'autre : il n'ecrit jamais.
package harvest

import (
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"github.com/UnPoilTefal/perimeter/internal/corpus"
	"github.com/UnPoilTefal/perimeter/internal/draft"
	"github.com/UnPoilTefal/perimeter/internal/perimeter"
)

// Element est une trace lue dans une source : un message de commit
// aujourd'hui, une revue de PR ou un post-mortem demain.
type Element struct {
	Ref   string
	Titre string
	Corps string
}

// Lecteur rend les traces d'une source. Injectable pour les tests, et pour
// brancher d'autres adaptateurs sans toucher au tri.
type Lecteur func(adapter, endpoint string, opts Options) ([]Element, error)

// Options regle la lecture et le volume rendu.
type Options struct {
	// AllowExec garde l'execution. Harvest lance des commandes de lecture :
	// il passe par la meme porte que le reste de l'outil, pour qu'aucun
	// chemin n'execute quoi que ce soit sans que ce soit demande.
	AllowExec bool
	// Depuis borne l'historique lu (revision ou date, au sens de git).
	Depuis string
	// Limite plafonne les candidats rendus. C'est un plafond, pas un seuil :
	// il borne le cout de relecture, il ne juge pas la pertinence.
	Limite int
	// Voisins est le nombre de notes proches rendues par candidat.
	Voisins int
	Lecteur Lecteur
}

const limiteParDefaut = 25

// Candidat est une proposition a relire. Jamais une note ecrite.
type Candidat struct {
	Source string
	Ref    string
	Titre  string
	// Signal est la phrase qui a declenche la retenue. Sans elle, ecarter un
	// faux demande de relire tout le corps.
	Signal string
	Corps  string
	// Trust vaut toujours « proposed » : voir toujoursProposed.
	Trust string
	// Voisins repond a « est-ce que ca existe deja ? », via la meme
	// machinerie que « perctl draft ». Ordonne, jamais conclusif : voir
	// voisinage pour la mesure qui l'impose.
	Voisins []draft.Voisin
}

// Result agrege une passe.
type Result struct {
	Source string
	// Lus est le nombre de traces parcourues, Retenus le nombre de candidats
	// avant plafonnement — les deux restent visibles pour que le plafond ne
	// cache pas ce qu'il a coupe.
	Lus       int
	Retenus   int
	Candidats []Candidat
}

// toujoursProposed : un harvest ne produit que du propose. Faire entrer du
// canon sans relecture, c'est exactement le mecanisme par lequel une
// hypothese fausse devient verite d'equipe.
const toujoursProposed = "proposed"

// motifPiege retient les formulations qui disent ce qui ne marche pas. C'est la ou
// vit le non re-derivable : un corps qui decrit ce que le commit fait se
// redemande au diff, il n'a rien a faire en memoire.
var motifPiege = regexp.MustCompile(`(?i)(sinon\b|au lieu de|ne suffit pas|n'existe pas` +
	`|ne .{0,24}(marche|fonctionne|tient|garde|prouve|protege|suffit)\b` +
	`|echoue|casse\b|piege|jamais\b|aucun\b|silence|invisible|faux (positif|negatif)` +
	`|contrairement|a tort|deguise|pourtant|alors que|attention)`)

// motifPourquoi retient l'explication. Exiger les deux signaux est un filtrage
// volontairement agressif : mesure sur un depot reel de 263 commits, 69 corps
// portent un piege, 38 un pourquoi, et 20 les deux.
var motifPourquoi = regexp.MustCompile(`(?i)(parce que|car\b|la raison|c'est que|d'ou\b|donc\b|des lors)`)

// Run lit la source d'etat declare du registre et propose des candidats.
func Run(c *corpus.Corpus, reg *perimeter.Registry, opts Options) (*Result, error) {
	if !opts.AllowExec {
		return nil, fmt.Errorf("harvest lit des sources par execution de commandes : relancer avec --allow-exec")
	}
	nom, src, err := sourceDeclaree(reg)
	if err != nil {
		return nil, err
	}
	lire := opts.Lecteur
	if lire == nil {
		lire = LireGit
	}
	elems, err := lire(src.Adapter, src.Endpoint, opts)
	if err != nil {
		return nil, err
	}

	limite := opts.Limite
	if limite <= 0 {
		limite = limiteParDefaut
	}

	res := &Result{Source: nom, Lus: len(elems)}
	for _, e := range elems {
		signal, ok := retient(e.Corps)
		if !ok {
			continue
		}
		res.Retenus++
		if len(res.Candidats) >= limite {
			continue
		}
		cand := Candidat{
			Source: nom, Ref: e.Ref, Titre: e.Titre,
			Signal: signal, Corps: strings.TrimSpace(e.Corps),
			Trust: toujoursProposed,
		}
		cand.Voisins = voisinage(c, e, signal, opts.Voisins)
		res.Candidats = append(res.Candidats, cand)
	}
	return res, nil
}

// sourceDeclaree est la porte de perimetre, et elle est structurelle : harvest
// ne lit que ce que le registre declare. Un transcript de session, lui,
// enregistre tout ce qui s'est dit devant l'agent — perimetre ou non — et
// demandera une porte explicite avant d'etre lu.
func sourceDeclaree(reg *perimeter.Registry) (string, perimeter.Source, error) {
	b, ok := reg.RoleMap["etat.declare"]
	if !ok {
		return "", perimeter.Source{}, fmt.Errorf("aucun role etat.declare dans le registre : harvest ne lit que des sources declarees")
	}
	for _, liaison := range b {
		src, ok := reg.Sources[liaison.Source]
		if !ok || src.Adapter != "git" {
			continue
		}
		if src.Endpoint == "" {
			return "", perimeter.Source{}, fmt.Errorf("la source %q n'a pas d'endpoint : rien a lire", liaison.Source)
		}
		return liaison.Source, src, nil
	}
	return "", perimeter.Source{}, fmt.Errorf("aucune source git rattachee a etat.declare : harvest ne sait lire que git pour l'instant")
}

func retient(corps string) (string, bool) {
	if strings.Count(strings.TrimSpace(corps), "\n") < 1 {
		return "", false
	}
	if !motifPourquoi.MatchString(corps) {
		return "", false
	}
	for _, ligne := range strings.Split(corps, "\n") {
		if m := motifPiege.FindString(ligne); m != "" {
			return strings.TrimSpace(ligne), true
		}
	}
	return "", false
}

// voisinage reutilise la machinerie de draft : la question « est-ce que ca
// existe deja ? » est la meme pour un brouillon humain et pour un candidat
// moissonne.
func voisinage(c *corpus.Corpus, e Element, signal string, plafond int) []draft.Voisin {
	// Titre + phrase-signal, et non le corps entier : la machinerie compare
	// des descriptions, et un corps de commit est dix fois plus long. Mesure
	// sur un depot reel — avec le corps entier, la note qui redit exactement
	// le candidat sortait bien en tete mais a 0,08, sous tout seuil utile ;
	// l'echelle ne transfere pas, seul le rang transfere.
	faux := &corpus.Note{
		Rel: "candidat.md", Base: "candidat.md",
		Desc: e.Titre + " " + signal,
	}
	// Aucun marqueur « deja connu » n'est rendu, et c'est mesure : sur 20
	// candidats d'un depot reel, un seuil a 0,15 marquait un vrai doublon,
	// un faux, et en manquait un autre — dont la note sortait pourtant en
	// tete a 0,12. Median 0,095, max 0,200 : rien ne separe. Le rang est
	// exploitable, le score ne l'est pas ; trancher ici donnerait un verdict
	// que la mesure ne soutient pas.
	return draft.Voisinage(c, faux, plafond)
}

// LireGit rend les messages de commit d'un depot.
func LireGit(adapter, endpoint string, opts Options) ([]Element, error) {
	if adapter != "git" {
		return nil, fmt.Errorf("adaptateur %q non gere par harvest", adapter)
	}
	args := []string{"-C", endpoint, "log", "--no-merges", "--format=%H%x1e%s%x1e%b%x1f"}
	if opts.Depuis != "" {
		args = append(args, opts.Depuis+"..HEAD")
	}
	ctx, annule := contexte()
	defer annule()
	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("lecture de %s : %w", endpoint, err)
	}
	var elems []Element
	for _, bloc := range strings.Split(string(out), "\x1f") {
		bloc = strings.TrimSpace(bloc)
		if bloc == "" {
			continue
		}
		parts := strings.SplitN(bloc, "\x1e", 3)
		for len(parts) < 3 {
			parts = append(parts, "")
		}
		elems = append(elems, Element{Ref: parts[0][:min(8, len(parts[0]))], Titre: parts[1], Corps: parts[2]})
	}
	return elems, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
