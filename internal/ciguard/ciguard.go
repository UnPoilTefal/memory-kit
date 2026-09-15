// Package ciguard decide si l'execution de preuves est sure dans le contexte
// courant.
//
// Les preuves sont du shell declare dans des fichiers markdown. Les executer
// sur une pull request venant d'un depot tiers revient a executer du code
// arbitraire fourni par l'exterieur. La regle existait depuis le debut — dans
// le README et dans un commentaire de workflow — mais rien ne l'appliquait :
// un pipeline configure correctement puis modifie plus tard par quelqu'un qui
// ignore la contrainte executait sans que rien ne le signale.
//
// Le principe retenu : le doute ne profite pas a l'execution. Sur un evenement
// de pull request, il faut pouvoir *confirmer* que la source est le depot
// lui-meme ; a defaut, on refuse.
package ciguard

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Verdict dit ce que le contexte autorise.
type Verdict struct {
	// Trusted vaut faux quand l'execution doit etre refusee.
	Trusted bool
	// Context nomme le contexte reconnu, pour que le message soit utile.
	Context string
	// Reason explique un refus.
	Reason string
}

// Env permet d'injecter la lecture d'environnement dans les tests.
type Env func(string) string

// ReadFile permet d'injecter la lecture du fichier d'evenement.
type ReadFile func(string) ([]byte, error)

// Assess rend le verdict du contexte courant.
func Assess(env Env, read ReadFile) Verdict {
	switch {
	case env("GITHUB_ACTIONS") == "true":
		return github(env, read)
	case env("GITLAB_CI") == "true":
		return gitlab(env)
	case env("CI") != "" && env("CI") != "false":
		// Un systeme d'integration qu'on ne sait pas interroger. Sans
		// indication d'evenement, rien ne permet d'affirmer qu'il s'agit
		// d'une contribution externe : on n'invente pas un risque.
		return Verdict{Trusted: true, Context: "integration continue non reconnue"}
	default:
		return Verdict{Trusted: true, Context: "hors integration continue"}
	}
}

func github(env Env, read ReadFile) Verdict {
	ev := env("GITHUB_EVENT_NAME")
	if ev != "pull_request" && ev != "pull_request_target" {
		return Verdict{Trusted: true, Context: "GitHub Actions, evenement " + ev}
	}
	ctx := "GitHub Actions, pull request"

	path := env("GITHUB_EVENT_PATH")
	if path == "" {
		return Verdict{Context: ctx,
			Reason: "GITHUB_EVENT_PATH absent : impossible de confirmer que la pull request vient de ce depot"}
	}
	raw, err := read(path)
	if err != nil {
		return Verdict{Context: ctx,
			Reason: fmt.Sprintf("charge d'evenement illisible (%v) : provenance de la pull request indeterminable", err)}
	}
	var payload struct {
		PullRequest struct {
			Head struct {
				Repo struct {
					FullName string `json:"full_name"`
				} `json:"repo"`
			} `json:"head"`
		} `json:"pull_request"`
		Repository struct {
			FullName string `json:"full_name"`
		} `json:"repository"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Verdict{Context: ctx,
			Reason: "charge d'evenement non analysable : provenance de la pull request indeterminable"}
	}
	source := payload.PullRequest.Head.Repo.FullName
	cible := payload.Repository.FullName
	switch {
	case source == "" || cible == "":
		return Verdict{Context: ctx,
			Reason: "la charge d'evenement ne nomme pas les depots source et cible"}
	case !strings.EqualFold(source, cible):
		return Verdict{Context: ctx,
			Reason: fmt.Sprintf("pull request issue de %s, distinct de %s : le contenu vient de l'exterieur", source, cible)}
	default:
		return Verdict{Trusted: true, Context: ctx + " du depot lui-meme"}
	}
}

func gitlab(env Env) Verdict {
	if env("CI_PIPELINE_SOURCE") != "merge_request_event" {
		return Verdict{Trusted: true, Context: "GitLab CI, source " + env("CI_PIPELINE_SOURCE")}
	}
	ctx := "GitLab CI, merge request"
	source := env("CI_MERGE_REQUEST_SOURCE_PROJECT_PATH")
	cible := env("CI_PROJECT_PATH")
	switch {
	case source == "" || cible == "":
		return Verdict{Context: ctx,
			Reason: "projets source et cible non renseignes : provenance de la merge request indeterminable"}
	case !strings.EqualFold(source, cible):
		return Verdict{Context: ctx,
			Reason: fmt.Sprintf("merge request issue de %s, distinct de %s : le contenu vient de l'exterieur", source, cible)}
	default:
		return Verdict{Trusted: true, Context: ctx + " du projet lui-meme"}
	}
}

// AssessDefault interroge l'environnement reel.
func AssessDefault() Verdict { return Assess(os.Getenv, os.ReadFile) }
