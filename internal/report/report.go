// Package report normalise les constats produits par les commandes et leurs
// formats de sortie : humain pour le terminal, JSON pour un agent, annotations
// GitHub Actions pour la CI.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Severity ordonne les constats. Error fait echouer, Warn seulement en --strict.
type Severity string

const (
	Error Severity = "error"
	Warn  Severity = "warn"
	Info  Severity = "info"
)

// Finding est un constat unitaire attache a un fichier.
type Finding struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	File     string   `json:"file"`
	Message  string   `json:"message"`
	// Hint dit quoi faire, pas seulement ce qui ne va pas.
	Hint string `json:"hint,omitempty"`
}

// Result agrege les constats d'une execution.
type Result struct {
	Command  string    `json:"command"`
	Root     string    `json:"root"`
	Notes    int       `json:"notes"`
	Findings []Finding `json:"findings"`
	Stats    map[string]any `json:"stats,omitempty"`
}

// Add enregistre un constat.
func (r *Result) Add(f Finding) { r.Findings = append(r.Findings, f) }

// Count compte les constats d'une severite.
func (r *Result) Count(s Severity) int {
	n := 0
	for _, f := range r.Findings {
		if f.Severity == s {
			n++
		}
	}
	return n
}

// ExitCode rend 1 s'il y a de quoi echouer, 0 sinon.
func (r *Result) ExitCode(strict bool) int {
	if r.Count(Error) > 0 || (strict && r.Count(Warn) > 0) {
		return 1
	}
	return 0
}

func (r *Result) sorted() []Finding {
	out := append([]Finding(nil), r.Findings...)
	rank := map[Severity]int{Error: 0, Warn: 1, Info: 2}
	sort.SliceStable(out, func(i, j int) bool {
		if rank[out[i].Severity] != rank[out[j].Severity] {
			return rank[out[i].Severity] < rank[out[j].Severity]
		}
		if out[i].Rule != out[j].Rule {
			return out[i].Rule < out[j].Rule
		}
		return out[i].File < out[j].File
	})
	return out
}

// Write rend le resultat dans le format demande.
func (r *Result) Write(w io.Writer, format string, color bool) error {
	switch format {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(r)
	case "github":
		for _, f := range r.sorted() {
			var err error
			lvl := "error"
			if f.Severity != Error {
				lvl = "warning"
			}
			msg := f.Message
			if f.Hint != "" {
				msg += " — " + f.Hint
			}
			if _, err = fmt.Fprintf(w, "::%s file=%s,title=%s::%s\n", lvl, f.File, f.Rule, strings.ReplaceAll(msg, "\n", " ")); err != nil {
				return err
			}
		}
		return nil
	default:
		return r.writeHuman(w, color)
	}
}

type palette struct{ red, yellow, dim, bold, reset string }

func newPalette(on bool) palette {
	if !on {
		return palette{}
	}
	return palette{red: "\x1b[31m", yellow: "\x1b[33m", dim: "\x1b[2m", bold: "\x1b[1m", reset: "\x1b[0m"}
}

func (r *Result) writeHuman(w io.Writer, color bool) error {
	p := newPalette(color)
	if len(r.Findings) == 0 {
		_, err := fmt.Fprintf(w, "%s✓%s %d notes, aucun constat\n", p.bold, p.reset, r.Notes)
		return err
	}

	byRule := map[string][]Finding{}
	var order []string
	for _, f := range r.sorted() {
		if _, seen := byRule[f.Rule]; !seen {
			order = append(order, f.Rule)
		}
		byRule[f.Rule] = append(byRule[f.Rule], f)
	}

	for _, rule := range order {
		fs := byRule[rule]
		mark, col := "!", p.yellow
		if fs[0].Severity == Error {
			mark, col = "x", p.red
		}
		fmt.Fprintf(w, "\n%s%s %s%s %s(%d)%s\n", col, mark, rule, p.reset, p.dim, len(fs), p.reset) //nolint:errcheck // sortie terminal
		if fs[0].Hint != "" {
			fmt.Fprintf(w, "  %s%s%s\n", p.dim, fs[0].Hint, p.reset) //nolint:errcheck // sortie terminal
		}
		for _, f := range fs {
			fmt.Fprintf(w, "    %s  %s\n", f.File, f.Message) //nolint:errcheck // sortie terminal
		}
	}

	_, err := fmt.Fprintf(w, "\n%s%d notes%s — %s%d erreurs%s, %s%d avertissements%s\n",
		p.bold, r.Notes, p.reset, p.red, r.Count(Error), p.reset, p.yellow, r.Count(Warn), p.reset)
	return err
}
