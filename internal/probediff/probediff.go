// Package probediff repere les preuves qui ont change depuis une reference.
//
// CODEOWNERS fait relire un fichier entier. Rien ne signale qu'un bloc de
// preuve a bouge entre deux revues — alors que c'est la seule portion du diff
// qui finira par executer du code. Un relecteur qui survole une note de 400
// mots ne voit pas necessairement que trois lignes de « verify: » ont change.
package probediff

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/UnPoilTefal/perimeter/internal/corpus"
)

// ReadAtRef rend le contenu d'un fichier a une reference donnee. Une erreur
// os.ErrNotExist signifie que la note n'existait pas encore.
type ReadAtRef func(rel string) ([]byte, error)

// Change decrit l'evolution des preuves d'une note.
type Change struct {
	Note     string
	Ajoutees []string
	Retirees []string
	// Nouvelle dit si la note elle-meme est apparue depuis la reference.
	Nouvelle bool
}

// Count rend le nombre de preuves touchees.
func (c Change) Count() int { return len(c.Ajoutees) + len(c.Retirees) }

// Changed compare les preuves du corpus a leur etat a la reference.
//
// Une note modifiee dont les preuves n'ont pas bouge n'est pas rapportee :
// c'est tout l'interet, ne montrer que ce qui execute du code.
func Changed(c *corpus.Corpus, atRef ReadAtRef) ([]Change, error) {
	var out []Change
	for _, n := range c.Notes {
		if n.ParseErr != nil {
			continue
		}
		avant, err := probesAtRef(n.Rel, atRef)
		nouvelle := false
		switch {
		case err == nil:
		case os.IsNotExist(err):
			nouvelle = true
		default:
			return nil, fmt.Errorf("%s : %w", n.Rel, err)
		}

		apres := signatures(n.Metadata.Verify)
		ajoutees := difference(apres, avant)
		retirees := difference(avant, apres)
		if len(ajoutees) == 0 && len(retirees) == 0 {
			continue
		}
		out = append(out, Change{Note: n.Rel, Ajoutees: ajoutees, Retirees: retirees, Nouvelle: nouvelle})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Note < out[j].Note })
	return out, nil
}

func probesAtRef(rel string, atRef ReadAtRef) ([]string, error) {
	raw, err := atRef(rel)
	if err != nil {
		return nil, err
	}
	n, err := corpus.ParseNote(rel, raw)
	if err != nil || n.ParseErr != nil {
		// Une note illisible a la reference : on considere qu'elle ne portait
		// aucune preuve, donc tout ce qu'elle porte aujourd'hui est nouveau.
		return nil, nil
	}
	return signatures(n.Metadata.Verify), nil
}

// signatures rend une forme comparable de chaque preuve. Le champ « note » en
// est exclu : reformuler un commentaire n'est pas changer ce qui s'execute.
func signatures(checks []corpus.Check) []string {
	var s []string
	for _, c := range checks {
		exit := ""
		if c.ExpectExit != nil {
			exit = fmt.Sprintf(" exit=%d", *c.ExpectExit)
		}
		switch {
		case c.ViaSource():
			s = append(s, fmt.Sprintf("source=%s arg=%q attendu=%q%s", c.Source, c.Arg, c.ExpectStdout, exit))
		default:
			s = append(s, fmt.Sprintf("cmd=%q attendu=%q%s", c.Cmd, c.ExpectStdout, exit))
		}
	}
	sort.Strings(s)
	return s
}

func difference(a, b []string) []string {
	dans := make(map[string]bool, len(b))
	for _, x := range b {
		dans[x] = true
	}
	var out []string
	for _, x := range a {
		if !dans[x] {
			out = append(out, x)
		}
	}
	return out
}

// Resume rend une ligne lisible par un humain et par une annotation de CI.
func (c Change) Resume() string {
	var parts []string
	if c.Nouvelle {
		parts = append(parts, "note nouvelle")
	}
	if n := len(c.Ajoutees); n > 0 {
		parts = append(parts, fmt.Sprintf("%d preuve(s) ajoutee(s)", n))
	}
	if n := len(c.Retirees); n > 0 {
		parts = append(parts, fmt.Sprintf("%d retiree(s) ou modifiee(s)", n))
	}
	return strings.Join(parts, ", ")
}
