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
	// Qualifier distingue la source quand le role en porte plusieurs. Sans
	// lui, la relecture ne peut pas rattacher un candidat a son depot.
	Qualifier string
	Ref       string
	Titre     string
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

// Ignoree nomme une source declaree que harvest n'a pas lue, et pourquoi. Se
// taire sur elle presenterait une moisson partielle comme complete — plus
// dangereux qu'une moisson vide, parce que rien n'invite a verifier.
type Ignoree struct {
	Source string
	Raison string
}

// Result agrege une passe.
type Result struct {
	// Sources liste les sources effectivement lues.
	Sources  []string
	Ignorees []Ignoree
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

// Run lit les sources d'etat declare du registre et propose des candidats.
func Run(c *corpus.Corpus, reg *perimeter.Registry, opts Options) (*Result, error) {
	if !opts.AllowExec {
		return nil, fmt.Errorf("harvest lit des sources par execution de commandes : relancer avec --allow-exec")
	}
	lues, ignorees, err := sourcesDeclarees(reg)
	if err != nil {
		return nil, err
	}
	lire := opts.Lecteur
	if lire == nil {
		lire = LireGit
	}
	limite := opts.Limite
	if limite <= 0 {
		limite = limiteParDefaut
	}

	res := &Result{Ignorees: ignorees}
	// Les candidats sont d'abord rassembles par source, puis servis en
	// alternance : un plafond global consomme source par source affamerait
	// les dernieres, et un plafond par source ferait mentir le chiffre
	// annonce a l'utilisateur.
	parSource := make([][]Candidat, 0, len(lues))
	for _, d := range lues {
		elems, err := lire(d.Src.Adapter, d.Src.Endpoint, opts)
		if err != nil {
			return nil, err
		}
		res.Sources = append(res.Sources, d.Nom)
		res.Lus += len(elems)

		var retenus []Candidat
		for _, e := range elems {
			signal, ok := retient(e.Corps)
			if !ok {
				continue
			}
			res.Retenus++
			cand := Candidat{
				Source: d.Nom, Qualifier: d.Qualifier, Ref: e.Ref, Titre: e.Titre,
				Signal: signal, Corps: strings.TrimSpace(e.Corps),
				Trust: toujoursProposed,
			}
			cand.Voisins = voisinage(c, e, signal, opts.Voisins)
			retenus = append(retenus, cand)
		}
		parSource = append(parSource, retenus)
	}

	for tour := 0; len(res.Candidats) < limite; tour++ {
		servi := false
		for _, liste := range parSource {
			if tour >= len(liste) {
				continue
			}
			servi = true
			res.Candidats = append(res.Candidats, liste[tour])
			if len(res.Candidats) >= limite {
				break
			}
		}
		if !servi {
			break
		}
	}
	return res, nil
}

// declaree porte une source lisible du role etat.declare.
type declaree struct {
	Nom       string
	Qualifier string
	Src       perimeter.Source
}

// sourcesDeclarees est la porte de perimetre, et elle est structurelle :
// harvest ne lit que ce que le registre declare. Un transcript de session, lui,
// enregistre tout ce qui s'est dit devant l'agent — perimetre ou non — et
// demandera une porte explicite avant d'etre lu.
//
// Toutes les sources git du role sont rendues, pas la premiere : depuis que le
// role accepte plusieurs sources, n'en lire qu'une revient a moissonner un
// depot sur N sans le dire.
func sourcesDeclarees(reg *perimeter.Registry) ([]declaree, []Ignoree, error) {
	b, ok := reg.RoleMap["etat.declare"]
	if !ok {
		return nil, nil, fmt.Errorf("aucun role etat.declare dans le registre : harvest ne lit que des sources declarees")
	}
	var lues []declaree
	var ignorees []Ignoree
	for _, liaison := range b {
		src, ok := reg.Sources[liaison.Source]
		switch {
		case !ok:
			ignorees = append(ignorees, Ignoree{liaison.Source, "source absente du registre"})
		case src.Adapter != "git":
			ignorees = append(ignorees, Ignoree{liaison.Source, "adaptateur " + src.Adapter + " : harvest ne sait lire que git"})
		case src.Endpoint == "":
			ignorees = append(ignorees, Ignoree{liaison.Source, "aucun endpoint : rien a lire"})
		default:
			lues = append(lues, declaree{Nom: liaison.Source, Qualifier: qualifierOuDefaut(liaison), Src: src})
		}
	}
	if len(lues) == 0 {
		return nil, ignorees, fmt.Errorf("aucune source git lisible rattachee a etat.declare : harvest ne sait lire que git pour l'instant")
	}
	return lues, ignorees, nil
}

// qualifierOuDefaut : un role a source unique n'a pas de qualifier, mais la
// provenance doit rester nommee dans la sortie.
func qualifierOuDefaut(b perimeter.Binding) string {
	if b.Qualifier != "" {
		return b.Qualifier
	}
	return b.Source
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
