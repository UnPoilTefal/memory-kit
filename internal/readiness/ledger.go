package readiness

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// LedgerFile est le nom conventionnel du journal des passages.
const LedgerFile = "readiness-ledger.jsonl"

// DefaultWindow est le nombre de passages consecutifs sur lequel se juge la
// sortie de demarrage.
const DefaultWindow = 5

// Entry est un passage de porte consigne. Le journal est en ajout seul : on
// ne reecrit pas l'histoire des verdicts, sinon la mesure de maturite devient
// declarative et ne vaut plus rien.
type Entry struct {
	At         time.Time `json:"at"`
	Spec       string    `json:"spec"`
	Verdict    Verdict   `json:"verdict"`
	Escalation string    `json:"escalation,omitempty"`
	Assessment string    `json:"assessment,omitempty"`
}

// Append ajoute un passage au journal, en le creant au besoin.
func Append(path string, e Entry) error {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close() //nolint:errcheck // ecriture deja synchronisee ci-dessous

	line, err := json.Marshal(e)
	if err != nil {
		return err
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		return err
	}
	return f.Sync()
}

// ReadLedger lit le journal. Un journal absent n'est pas une erreur : c'est
// l'etat initial, et il signifie que le demarrage n'a pas commence.
func ReadLedger(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // lecture seule

	var entries []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if err := json.Unmarshal(line, &e); err != nil {
			return nil, fmt.Errorf("%s ligne %d : %w", path, n, err)
		}
		entries = append(entries, e)
	}
	return entries, sc.Err()
}

// Mode est le regime de la boucle d'apprentissage.
type Mode string

const (
	// Interactif : la boite propose et l'humain valide au fil de l'eau. C'est
	// le regime de demarrage, quand il n'y a encore rien a relire.
	Interactif Mode = "interactif"
	// Lot : la boite propose par lot, relu comme du code. C'est le regime
	// etabli, celui qui rend la connaissance relisible par l'equipe.
	Lot Mode = "lot"
)

// Maturity est l'etat de sortie de demarrage.
type Maturity struct {
	Window     int
	Considered int
	// KnowledgeEscalations compte, dans la fenetre, les passages ayant rendu
	// la main sur une carence de connaissance.
	KnowledgeEscalations int
	// Bootstrap dit si le demarrage est encore en cours.
	Bootstrap bool
	Mode      Mode
	Reason    string
}

// Assess rend l'etat de maturite depuis le journal.
//
// C'est la condition E3 : le demarrage prend fin quand, sur une fenetre de
// passages consecutifs, plus aucune escalade ne porte sur une carence de
// connaissance — seulement sur l'intention. La mesure est gratuite parce que
// la porte classe deja ses escalades ; l'instrument qui evalue les
// specifications evalue aussi sa propre maturite.
func Assess(entries []Entry, window int) Maturity {
	if window <= 0 {
		window = DefaultWindow
	}
	m := Maturity{Window: window}

	if len(entries) < window {
		m.Considered = len(entries)
		m.Bootstrap = true
		m.Mode = Interactif
		m.Reason = fmt.Sprintf("%d passage(s) consignes sur les %d requis : le demarrage n'a pas encore de quoi se juger",
			len(entries), window)
		return m
	}

	recent := entries[len(entries)-window:]
	m.Considered = len(recent)
	for _, e := range recent {
		if e.Escalation == Connaissance {
			m.KnowledgeEscalations++
		}
	}
	if m.KnowledgeEscalations > 0 {
		m.Bootstrap = true
		m.Mode = Interactif
		m.Reason = fmt.Sprintf("%d escalade(s) sur une carence de connaissance dans les %d derniers passages : les sources ne couvrent pas encore le perimetre",
			m.KnowledgeEscalations, window)
		return m
	}
	m.Bootstrap = false
	m.Mode = Lot
	m.Reason = fmt.Sprintf("aucune escalade de connaissance sur les %d derniers passages : le perimetre est couvert", window)
	return m
}
