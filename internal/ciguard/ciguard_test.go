package ciguard

import (
	"errors"
	"strings"
	"testing"
)

func env(m map[string]string) Env {
	return func(k string) string { return m[k] }
}

func charge(json string) ReadFile {
	return func(string) ([]byte, error) { return []byte(json), nil }
}

func illisible() ReadFile {
	return func(string) ([]byte, error) { return nil, errors.New("acces refuse") }
}

const memeDepot = `{"pull_request":{"head":{"repo":{"full_name":"org/produit"}}},"repository":{"full_name":"org/produit"}}`
const depuisFork = `{"pull_request":{"head":{"repo":{"full_name":"tiers/produit"}}},"repository":{"full_name":"org/produit"}}`

// Test 1 — hors integration continue, rien ne change.
func TestHorsCIRienNeChange(t *testing.T) {
	v := Assess(env(nil), charge(""))
	if !v.Trusted {
		t.Errorf("execution locale refusee a tort : %s", v.Reason)
	}
}

// Test 2 — un evenement qui n'est pas une pull request n'est pas suspect.
func TestEvenementNonPullRequestEstAutorise(t *testing.T) {
	for _, ev := range []string{"push", "schedule", "workflow_dispatch", "release"} {
		v := Assess(env(map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": ev}), charge(""))
		if !v.Trusted {
			t.Errorf("evenement %q refuse a tort : %s", ev, v.Reason)
		}
	}
}

// Test 3 — une pull request du depot lui-meme reste digne de confiance.
func TestPullRequestDuDepotLuiMemeEstAutorisee(t *testing.T) {
	v := Assess(env(map[string]string{
		"GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "pull_request",
		"GITHUB_EVENT_PATH": "/evenement.json",
	}), charge(memeDepot))
	if !v.Trusted {
		t.Errorf("pull request interne refusee a tort : %s", v.Reason)
	}
}

// Test 4 — une pull request venant d'un fork est refusee, et la raison nomme
// les deux depots.
func TestPullRequestDeForkEstRefusee(t *testing.T) {
	v := Assess(env(map[string]string{
		"GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "pull_request",
		"GITHUB_EVENT_PATH": "/evenement.json",
	}), charge(depuisFork))
	if v.Trusted {
		t.Fatal("une pull request de fork doit etre refusee")
	}
	if !strings.Contains(v.Reason, "tiers/produit") || !strings.Contains(v.Reason, "org/produit") {
		t.Errorf("la raison doit nommer les deux depots : %q", v.Reason)
	}
}

// « pull_request_target » est le plus dangereux des deux : il tourne avec les
// secrets du depot cible.
func TestPullRequestTargetEstTraiteCommeUnePullRequest(t *testing.T) {
	v := Assess(env(map[string]string{
		"GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "pull_request_target",
		"GITHUB_EVENT_PATH": "/evenement.json",
	}), charge(depuisFork))
	if v.Trusted {
		t.Error("pull_request_target depuis un fork doit etre refuse")
	}
}

// Test 5 — le doute ne profite pas a l'execution.
func TestProvenanceIndeterminableEstRefusee(t *testing.T) {
	base := map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "pull_request"}

	cas := []struct {
		nom  string
		env  map[string]string
		read ReadFile
	}{
		{"chemin d'evenement absent", base, charge("")},
		{"charge illisible", map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "pull_request", "GITHUB_EVENT_PATH": "/x.json"}, illisible()},
		{"charge non analysable", map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "pull_request", "GITHUB_EVENT_PATH": "/x.json"}, charge("{ pas du json")},
		{"depots non nommes", map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "pull_request", "GITHUB_EVENT_PATH": "/x.json"}, charge(`{"pull_request":{}}`)},
	}
	for _, c := range cas {
		if v := Assess(env(c.env), c.read); v.Trusted {
			t.Errorf("%s : devait etre refuse, le doute ne profite pas a l'execution", c.nom)
		}
	}
}

// --- GitLab ---

func TestMergeRequestDeForkEstRefusee(t *testing.T) {
	v := Assess(env(map[string]string{
		"GITLAB_CI": "true", "CI_PIPELINE_SOURCE": "merge_request_event",
		"CI_MERGE_REQUEST_SOURCE_PROJECT_PATH": "tiers/produit", "CI_PROJECT_PATH": "org/produit",
	}), charge(""))
	if v.Trusted {
		t.Fatal("une merge request de fork doit etre refusee")
	}
	if !strings.Contains(v.Reason, "tiers/produit") {
		t.Errorf("la raison doit nommer le projet source : %q", v.Reason)
	}
}

func TestMergeRequestDuProjetLuiMemeEstAutorisee(t *testing.T) {
	v := Assess(env(map[string]string{
		"GITLAB_CI": "true", "CI_PIPELINE_SOURCE": "merge_request_event",
		"CI_MERGE_REQUEST_SOURCE_PROJECT_PATH": "org/produit", "CI_PROJECT_PATH": "org/produit",
	}), charge(""))
	if !v.Trusted {
		t.Errorf("merge request interne refusee a tort : %s", v.Reason)
	}
}

func TestPipelineGitLabHorsMergeRequestEstAutorise(t *testing.T) {
	v := Assess(env(map[string]string{"GITLAB_CI": "true", "CI_PIPELINE_SOURCE": "schedule"}), charge(""))
	if !v.Trusted {
		t.Errorf("pipeline planifie refuse a tort : %s", v.Reason)
	}
}

// Une integration continue qu'on ne sait pas interroger ne donne aucune
// indication d'evenement : on n'invente pas un risque, on ne casse pas les
// travaux nocturnes legitimes.
func TestCINonReconnueNEstPasRefuseeParDefaut(t *testing.T) {
	v := Assess(env(map[string]string{"CI": "true"}), charge(""))
	if !v.Trusted {
		t.Errorf("une CI inconnue sans evenement de contribution ne doit pas etre refusee : %s", v.Reason)
	}
	if !strings.Contains(v.Context, "non reconnue") {
		t.Errorf("le contexte doit dire qu'il n'est pas reconnu : %q", v.Context)
	}
}

// Le contexte est nomme dans tous les cas : un refus doit etre diagnosticable.
func TestLeContexteEstToujoursNomme(t *testing.T) {
	cas := []Env{
		env(nil),
		env(map[string]string{"CI": "true"}),
		env(map[string]string{"GITHUB_ACTIONS": "true", "GITHUB_EVENT_NAME": "push"}),
		env(map[string]string{"GITLAB_CI": "true", "CI_PIPELINE_SOURCE": "push"}),
	}
	for i, e := range cas {
		if v := Assess(e, charge("")); v.Context == "" {
			t.Errorf("cas %d : contexte non nomme", i)
		}
	}
}
