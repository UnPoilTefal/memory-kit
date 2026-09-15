package perimeter

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Remarque est un constat de maturite : le registre est valide, mais quelque
// chose y affaiblit ce qu'on pourra en tirer.
//
// Pas de score, et c'est delibere. Le projet a remplace les scores de
// confiance par des tests falsifiables, et un « registre a 78 % » inviterait a
// optimiser le chiffre plutot que le perimetre. Chaque remarque porte donc un
// motif nommable et sa consequence — et le silence quand il n'y a rien a dire.
type Remarque struct {
	Motif       string
	Role        string
	Source      string
	Message     string
	Consequence string
}

func (r Remarque) String() string {
	sujet := r.Role
	if sujet == "" {
		sujet = "source " + r.Source
	}
	return fmt.Sprintf("%s : %s\n    %s", sujet, r.Message, r.Consequence)
}

// familles regroupe les roles par nature. Une meme source servant deux
// familles est valide, mais le perimetre cesse alors de les distinguer.
func famille(role string) string {
	if i := strings.Index(role, "."); i > 0 {
		return role[:i]
	}
	return role
}

// existenceSeule reconnait les sondes qui ne prouvent que la presence de la
// source. Elles etablissent qu'elle est la, pas qu'elle dit ce qu'on attend
// d'elle — meme motif qu'une garde qui invoque ce qu'elle protege.
var existenceSeule = regexp.MustCompile(`^\s*(true|test\s+-[defsr]\b[^&|;]*|ls\b[^&|;]*|stat\b[^&|;]*)\s*$`)

// Maturite rend les constats non bloquants sur le registre. Ils ne sont pas
// des erreurs : un registre qui en porte reste valide, et c'est voulu — les
// signaler comme des erreurs rendrait invalide tout registre existant et
// ferait desactiver la verification en entier.
func (r *Registry) Maturite() []Remarque {
	var out []Remarque

	// L'etat reel non mesure est l'angle mort meme que l'outil existe pour
	// reveler : si le reel est « declare », il n'est pas constate, et la
	// divergence entre le declare et le reel devient inobservable.
	for _, liaison := range r.RoleMap["etat.reel"] {
		src, ok := r.Sources[liaison.Source]
		if !ok || src.Reliability == "measured" {
			continue
		}
		out = append(out, Remarque{
			Motif: "etat-reel-non-mesure", Role: "etat.reel", Source: liaison.Source,
			Message:     fmt.Sprintf("la source %q porte reliability: %s", liaison.Source, src.Reliability),
			Consequence: "le reel n'est pas constate mais declare : l'ecart entre les deux verites devient inobservable",
		})
	}

	// Une source partagee entre deux familles de roles.
	parSource := map[string]map[string]bool{}
	for role, liaisons := range r.RoleMap {
		for _, l := range liaisons {
			if parSource[l.Source] == nil {
				parSource[l.Source] = map[string]bool{}
			}
			parSource[l.Source][famille(role)] = true
		}
	}
	for source, fams := range parSource {
		if len(fams) < 2 {
			continue
		}
		var noms []string
		for f := range fams {
			noms = append(noms, f)
		}
		sort.Strings(noms)
		out = append(out, Remarque{
			Motif: "familles-confondues", Source: source,
			Message:     fmt.Sprintf("sert %s", strings.Join(noms, " et ")),
			Consequence: "le perimetre ne distingue plus ce qui est voulu de ce qui est : les comparer n'apprend rien",
		})
	}

	for nom, src := range r.Sources {
		if src.Probe == nil || !existenceSeule.MatchString(src.Probe.Cmd) {
			continue
		}
		out = append(out, Remarque{
			Motif: "sonde-d-existence", Source: nom,
			Message:     fmt.Sprintf("la sonde se limite a %q", strings.TrimSpace(src.Probe.Cmd)),
			Consequence: "elle prouve que la source est la, pas qu'elle dit ce qu'on attend d'elle",
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Motif != out[j].Motif {
			return out[i].Motif < out[j].Motif
		}
		return out[i].Source < out[j].Source
	})
	return out
}
