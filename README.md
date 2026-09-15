# perimeter

Savoir si un agent peut agir — sur un homelab, une équipe, un produit.

Un **périmètre**, c'est l'ensemble de ce qu'un agent doit connaître pour travailler sans se
tromper de prémisse : une mémoire de ce qui n'est pas re-dérivable, un registre des briques qui
font autorité, et une porte qui décide si une spécification est prête.

Ce qu'un agent tient pour acquis pourrit par défaut, et pourrit en silence. `perimeter` traite
cette connaissance comme du code : schéma, CI, revue, et surtout **des faits qui portent leur
propre preuve**.

```
perctl lint       # la structure du corpus tient-elle ?
perctl verify     # les faits et les sources sont-ils encore vrais ?
perctl perimeter  # chaque rôle est-il pourvu et sondable ?
perctl gate       # cette spécification est-elle prête ?
```

---

## L'axiome

> **La mémoire ne stocke que ce qui n'est pas re-dérivable depuis la source de vérité.**

Ce qui est dans le code, l'état d'infrastructure, le système vivant ou l'historique git
se redemande à l'exécution. Ce qui reste tient dans peu de place et ne pourrit presque
pas : **le pourquoi et les pièges**. C'est cette frontière qui borne la taille du corpus
et qui rend la péremption gérable.

Corollaire pratique : une note qui commence par « le service X écoute sur le port Y »
est probablement à jeter. Une note qui dit « le port 9 de cette gateway est le WAN2,
indiscernable d'un port LAN libre dans l'interface » ne se re-dérive pas — elle a coûté
une panne.

## Les cinq registres

Un corpus mélange cinq choses qui n'ont ni le même cycle de vie, ni le même relecteur.
Les confondre dans un wiki unique est le mode d'échec le plus courant.

| Registre | Contenu | Durée de vie |
|---|---|---|
| `user` | qui est l'utilisateur, son contexte | lente |
| `feedback` | comment on attend que l'agent travaille | lente |
| `reference` | un fait contre-intuitif, un piège | stable, **vérifiable** |
| `project` | un chantier en cours, son état | **courte** |
| `decision` | un choix et son pourquoi (ADR) | immuable |

`project` est le registre le plus périssable : c'est là que la pourriture commence.

---

## Installation

```bash
brew install UnPoilTefal/tap/perimeter
```

<details>
<summary>Autres canaux</summary>

```bash
# Binaires : https://github.com/UnPoilTefal/perimeter/releases

# Depuis les sources (suppose une chaîne Go)
go install github.com/UnPoilTefal/perimeter/cmd/perctl@latest

# En CI, sans installer Go
docker run --rm -v "$PWD:/w" -w /w ghcr.io/unpoiltefal/perimeter:latest lint memory/
```

</details>

## Démarrage

```bash
cd <votre corpus>
perctl init      # écrit .corpus.yml
perctl lint      # premier état des lieux
```

Un corpus est un répertoire de notes markdown à frontmatter YAML, plus un index qui
sert de routeur. La disposition par défaut correspond à celle d'un répertoire de
mémoire Claude Code : les notes à plat, `MEMORY.md` en index.

### Anatomie d'une note

```markdown
---
name: reference-udm-port-9-est-wan2
description: "Le port 9 de l'UDM Pro est le WAN2, indiscernable d'un port LAN libre
  dans l'interface : y brancher un équipement le place sur le WAN"
metadata:
  type: reference
  trust: canon
  owner: "@equipe-reseau"
  source: "https://github.com/org/repo/issues/24"
  modified: 2026-09-13
  verify:
    - cmd: "curl -s ... | jq -r '.ports[8].name'"
      expect_stdout: "WAN2"
      note: "le port 9 est toujours déclaré WAN côté gateway"
---

Un fait, une note. Le corps explique ce que la description annonce,
et lie les notes voisines avec [[reference-autre-note]].
```

Le schéma complet : [`schema/memory.schema.json`](schema/memory.schema.json)
(`perctl schema` l'écrit sur la sortie standard, pour votre éditeur).

---

## `perctl lint` — la structure

| Règle | Sévérité | Ce qu'elle empêche |
|---|---|---|
| `parse` | erreur | une note illisible est invisible à tout le reste |
| `schema` | erreur | frontmatter non conforme |
| `name-match` | erreur | `name` divergent du fichier : les liens pointent dans le vide |
| `index-orphan` | erreur | **une note absente de l'index est écrite mais jamais lue** |
| `index-dangling` | erreur | l'index cite une note supprimée |
| `index-drift` | avert. | l'accroche d'index contredit la description qu'elle route |
| `wikilink` | avert. | lien cassé, y compris vers un corpus voisin |
| `description` | avert. | une description qui ne permet pas de décider du rappel |
| `atomicity` | avert. | une note trop large ne se périme jamais proprement |
| `ownership` | avert. | sans propriétaire, personne ne supprime jamais rien |
| `staleness` | avert. | une note dont l'échéance de relecture est passée |
| `staleness-budget` | erreur | le corpus dérive plus vite qu'il n'est relu |
| `secret` | erreur | la mémoire est rechargée à chaque session : c'est un vecteur de fuite |

`index-orphan` est la règle qui justifie l'outil à elle seule. L'index est le routeur du
rappel : une note qui n'y figure pas a été écrite, relue, commitée — et ne sera jamais
lue par l'agent. Rien ne le signale sans outillage. Sur le corpus qui a servi à
développer `perimeter`, 11 notes sur 98 étaient dans ce cas.

```bash
perctl lint                      # sortie terminal
perctl lint --format github      # annotations GitHub Actions
perctl lint --format json        # pour un agent
perctl lint --strict             # les avertissements font échouer
perctl lint --disable atomicity,staleness
```

### L'index est de l'état dérivé

L'accroche portée par l'index doit reproduire la description de la note. Quand elle est
maintenue à la main en parallèle, elle finit par annoncer autre chose que ce que la note
dit — et le rappel se fait sur une information fausse.

```bash
perctl index           # quelles notes manquent à l'index
perctl index --fix     # les ajouter
perctl index --sync    # régénérer les accroches depuis les descriptions
```

### Proposer les sondes plutôt que les attendre

Le mécanisme de preuve ne sert à rien tant qu'il faut écrire chaque sonde à la main. Sur le
corpus de développement, **2 notes sur 101** en portaient une.

```bash
perctl propose            # ce qu'il écrirait
perctl propose --write    # l'écrit, en commentaire
```

`propose` type chaque assertion à partir de **signaux structurels ancrés**, jamais du sens du
texte, et propose la sonde correspondante :

| Confiance | Signal | Sonde proposée |
|---|---|---|
| `registre` | la note cite une source déclarée — `produit#412` | `source: tickets`, `arg: "412"` |
| `structurel` | chemin absolu, URL, hôte, version | `test -e …`, `curl -sfI …` |

L'ancrage au registre est le bon : **ne pas deviner ce qu'est un dépôt, demander au registre.**
Un motif générique (`mot/mot`) produit 80 % de faux positifs, et un endpoint dégénéré (`.`)
donne une ancre qui matche tout — les deux mesurés, les deux écartés par le code.

**Un signal ne compte que s'il est proche de la revendication** : dans la `description` ou le
premier paragraphe. Au-delà, c'est du contexte. Sans cette contrainte, 45 notes d'un corpus de
101 portaient un signal pour une précision de 20 % après tri humain ; avec, 21 notes et ~38 %.
Le coût est assumé — une bonne proposition sur neuf est perdue, celle dont le chemin n'apparaît
qu'au troisième paragraphe.

**Une note qui énonce une absence appelle une sonde inversée.** « Ce chemin n'existe plus »
se prouve par `! test -e`, pas par `test -e` — qui échouerait alors que la note dit vrai. La
détection est locale à la phrase portant le signal, et n'accepte que des marqueurs francs :
« jamais » et « aucun » visaient 24 notes sur 101, dont la plupart n'énoncent aucune absence.

Chaque type porte sa **demi-vie** : un chemin se périme plus vite qu'une convention.

**Deux règles de sûreté, chacune née d'une erreur constatée :**

- Une sonde n'est **jamais** construite à partir d'une commande citée dans une note. On
  n'exécute pas ce qu'on trouve écrit.
- Les sondes sont écrites **en commentaire**. Une note ne devient vérifiable que lorsqu'un
  humain a relu et décommenté — même discipline que pour la mémoire : une proposition d'agent
  arrive en `proposed`, jamais en `canon`.

`propose` annonce toujours **le corpus et le registre sur lesquels il agit**. Un registre au
chemin absolu fait travailler ailleurs que là où l'on croit être.

---

## `perctl verify` — les faits

C'est la boucle qui sépare un corpus de mémoire d'un wiki. Un fait qui porte une commande
de vérification cesse d'être une affirmation datée : il devient une assertion testable.

```yaml
metadata:
  verify:
    - cmd: "gh api repos/org/repo/rulesets --jq '.[].name'"
      expect_stdout: "protect-main"
      note: "la protection de branche tient toujours"
```

```bash
perctl verify --allow-exec           # rejoue les preuves
perctl verify --allow-exec --write   # inscrit verified_at et verify_status
perctl verify --allow-exec --only authentik
```

Une preuve en échec **ne supprime pas la note** : elle la signale. C'est un humain qui
tranche entre « le fait a changé » et « le monde a changé » — les deux se corrigent
différemment.

`verify` rapporte aussi le **taux de couverture** : quelle proportion du corpus porte une
preuve. C'est la métrique de santé la plus utile, et elle monte lentement — commencez par
les faits qui vous ont déjà coûté un incident.

### Sécurité : `verify` exécute du code

Les commandes sont du shell déclaré dans des fichiers de documentation. C'est délibéré —
c'est ce qui rend la preuve exécutable — et c'est une surface d'attaque.

- L'exécution est **refusée par défaut**. `--allow-exec` est obligatoire.
- Un corpus partagé doit protéger ses notes par `CODEOWNERS`, au même titre que sa CI.
- Le job CI qui lance `verify` doit avoir des permissions minimales et **ne jamais tourner
  sur une pull request venant d'un fork**.
- N'écrivez que des commandes en lecture seule et idempotentes.

`perctl lint` ne lance jamais rien : la CI de pull request peut l'utiliser sans réserve.

---

## Le registre de périmètre

Un corpus de mémoire ne vit pas seul. Autour de lui, une équipe a des tickets, des décisions,
des dépôts, un état réel — chacun avec son accès, ses identifiants, et surtout **son régime de
fiabilité**. Le registre déclare quelle brique concrète sert quel rôle.

```yaml
# perimeter.yml
version: 1

roles:
  intention.spec:        { source: specs }
  intention.tickets:     { source: tickets }
  contrainte.decisions:  { source: adr }
  contrainte.memoire:    { source: memoire }
  etat.declare:          { source: depots }
  etat.reel:             { source: cluster }

sources:
  tickets:
    adapter: github
    endpoint: org/produit
    reliability: measured
    credential: env:GITHUB_TOKEN      # une référence, jamais la valeur
    probe:
      cmd: "gh api repos/${endpoint} --jq .full_name"
      expect_stdout: "^org/produit$"
    query:
      cmd: "gh api repos/${endpoint}/issues/${arg} --jq .state"
```

```bash
perctl perimeter perimeter.yml   # chaque rôle est-il pourvu, chaque source sondable ?
```

### Les six rôles ne sont pas configurables

`intention.spec`, `intention.tickets`, `contrainte.decisions`, `contrainte.memoire`,
`etat.declare`, `etat.reel`. Un périmètre qui n'en pourvoit pas un a une **carence**, pas une
préférence — et le registre la nomme plutôt que de la laisser passer. C'est le premier signal
qu'un périmètre n'est pas encore décrit.

### Les sources portent leur propre sonde

C'est l'axiome remonté d'un cran. Sans sonde, une boîte qui n'a vérifié que ses *faits* finit
par affirmer sereinement qu'aucun ticket ne contredit — parce que son jeton a expiré trois
semaines plus tôt.

```bash
perctl verify <corpus> --perimeter perimeter.yml --probe-sources --allow-exec
```

La vérification porte alors sur **les faits, les sources, et le registre**.

### Une preuve peut renvoyer à une source

```yaml
metadata:
  verify:
    - source: tickets          # l'interrogation est résolue par le registre
      arg: "412"
      expect_stdout: "^closed$"
```

Préférable à une commande écrite en dur dès qu'un périmètre est déclaré : le fait survit au
changement d'outil, la commande non. Changer de traqueur de tickets devient une ligne du
registre au lieu d'une reprise de toutes les notes.

### Deux invariants, appliqués par le schéma et non par la discipline

**Aucun identifiant en clair.** Le champ `credential` n'accepte qu'une référence
(`env:` `file:` `keychain:` `cmd:` `op:`), résolue à l'exécution. Une valeur déguisée en
référence est signalée à la validation.

**Aucune recopie de source.** Le schéma n'offre aucun champ de cache, de miroir ou de
synchronisation locale : en déclarer un est une erreur de validation. Ce qui est dérivable est
cherché au moment de l'évaluation, jamais mémorisé — sinon on recrée exactement la péremption
qu'on cherche à supprimer.

### Compatibilité

Un corpus sans registre garde **exactement** son comportement : `perctl lint` et
`perctl verify` fonctionnent comme avant. Le registre est additif.

Une preuve qui renvoie à une source alors qu'aucun registre n'est chargé **échoue nommément**
plutôt que d'être ignorée — une preuve non jouée ne prouve rien, et la taire donnerait un
verdict faussement vert.

---

## La porte de readiness

Une spécification n'est pas prête parce qu'on se sent confiant. Elle est prête quand on sait
écrire les tests qui diront qu'elle est satisfaite.

> **L'incapacité à écrire le test est le signal.**

On ne sait pas exprimer l'assertion d'une intention qu'on n'a pas comprise — et le blocage est
localisé : on sait *quel* test résiste, donc quelle partie de la spécification est trouble. Le
même artefact sert deux fois : **porte** avant de produire, **critère d'acceptation** après.

### Le partage du travail

`perctl` ne juge pas si une spécification est comprise — c'est un binaire, il ne sait pas lire.
Il fournit **le cadre et le registre des passages** ; l'agent fournit le jugement. C'est la même
division que pour les preuves : quelqu'un écrit la sonde, l'outil la rejoue.

Le verdict n'est donc **jamais déclaré** — le schéma n'accepte aucun champ `verdict`. Il est
dérivé de ce qui a été consigné.

```yaml
# evaluation.yml
version: 1
spec: "issue #131 — alerter quand une source ne répond plus"

tests:
  - id: sonde-en-echec-signale
    assertion: "une source dont la sonde échoue produit un constat nommant la source"
    status: writable
  - id: seuil-de-bruit
    assertion: "une source en échec depuis moins que le seuil toléré n'alerte pas"
    status: blocked
    blocked_by: seuil-inconnu

deficiencies:
  - id: seuil-inconnu
    classification: mesurable
    statement: "le seuil de tolérance n'est pas connu — il se lit dans l'historique du journal"

facts:
  - reference-index-est-le-routeur
```

```bash
perctl gate evaluation.yml --perimeter perimeter.yml --corpus memory/
```

### Trois classifications, trois issues

| Classification | Issue | Régime |
|---|---|---|
| `mesurable` | **instruire** — l'agent va la combler seul | normal |
| `connaissance` | **rendre la main** — le savoir n'est dans aucune source déclarée | démarrage |
| `intention` | **rendre la main** — seul arrêt légitime en régime établi | permanent |

Le critère n'est pas « suis-je incertain » mais **« est-ce réductible par déduction »**. Tout ce
qui est mesurable, l'agent va le chercher ; seule l'ambiguïté d'intention justifie de rendre la
main. Le code de sortie porte le verdict — `0` produire, `1` instruire, `2` rendre la main — pour
qu'une automatisation s'y branche.

### Ce que l'outil vérifie seul

Deux préconditions mécaniques, qui empêchent `produire` sans jamais faire rendre la main (par
construction, l'agent peut aller les régler) :

- **la couverture du périmètre** — un rôle non pourvu, c'est E1 non tenue ;
- **la fraîcheur des faits mobilisés** — s'appuyer sur un fait dont la preuve ne tient plus,
  c'est agir sur une prémisse fausse, exactement ce que la porte existe pour empêcher.

Et une cohérence : un test bloqué doit désigner une carence existante, une carence doit bloquer
quelque chose ou être résolue. Sans ça, omettre de classer ce qui bloque produirait un verdict
faussement vert.

### Sortir du démarrage

Chaque passage est consigné dans un journal **en ajout seul** — on ne réécrit pas l'histoire des
verdicts, sinon la mesure de maturité devient déclarative et ne vaut plus rien.

```bash
perctl readiness
```

Le démarrage prend fin quand, sur une fenêtre de passages consécutifs, **plus aucune escalade ne
porte sur une carence de connaissance** — seulement sur l'intention. La mesure est gratuite : la
porte classe déjà ses escalades. L'instrument qui évalue les spécifications évalue aussi sa
propre maturité.

**Le régime n'est pas un réglage.** Il se déduit du journal : interactif tant que le démarrage
dure, par lot ensuite. Aucun drapeau à positionner, rien qui pourrisse.

Si le journal se remplit sans jamais tenir la condition, ce n'est pas une boucle à laisser
tourner — c'est un diagnostic : *les sources déclarées ne suffisent pas à couvrir le périmètre*.

---

## Mode d'emploi

### Le portillon d'écriture

Trois questions avant d'écrire quoi que ce soit. Une seule réponse négative, on n'écrit pas.

1. **Est-ce non re-dérivable ?** Si un appel d'outil donne la réponse, ce n'est pas de la mémoire.
2. **Est-ce non éphémère ?** Si c'est vrai cette semaine seulement, c'est une issue, pas une mémoire.
3. **Est-ce que ça comptera dans trois mois ?** Sinon, le coût de relecture dépasse la valeur.

Plus une contrainte dure : **un fait par fichier**. C'est ce qui rend le dédoublonnage et
la péremption praticables.

### Écrire la description

C'est la description, pas le corps, que l'agent lit pour décider si la note est
pertinente. Une mauvaise description rend la note aussi inutile qu'une note absente de
l'index.

- Énoncer **le fait**, pas le sujet. `« Renovate retargète en interne la dernière
  version, ce qui fige une PR en pending »` et non `« notes sur Renovate »`.
- Y mettre les mots qu'on emploierait en cherchant, pas le vocabulaire canonique.
- Une phrase. Si deux sont nécessaires, la note n'est pas atomique.

### Le rituel

| Quand | Quoi |
|---|---|
| À chaque PR | `perctl lint --format github` |
| Chaque nuit | `perctl verify --allow-exec --write`, PR d'écart si échec |
| Chaque mois | relire les `project` — c'est le registre qui pourrit |
| Chaque trimestre | purger ce qui n'a jamais été rappelé |

---

## En équipe

Quatre choses qui ne servent à rien quand on est seul, et sans lesquelles le corpus
d'une équipe meurt en six mois.

**Niveaux de confiance.** `trust: canon | proposed | personal`. `canon` a été revu en PR
par le propriétaire du domaine ; `proposed` a été écrit par un agent et ne l'a pas été ;
`personal` n'a qu'une portée individuelle. L'agent doit savoir dans quel niveau il puise.
Sans ça, l'hypothèse fausse d'une personne devient vérité d'équipe en trois semaines.

**Propriété.** `owner` sur chaque note, plus `CODEOWNERS` sur le répertoire. Activer
`policy.require_owner`. Sans propriétaire désigné, personne ne supprime jamais rien, et
le corpus meurt par accumulation.

**Cloisonnement.** La règle `secret` est un garde-fou avant le commit, pas un scanner :
doublez-la de `gitleaks` en CI. Et fixez explicitement ce qui n'a pas le droit d'entrer —
client, données personnelles, périmètre.

**Une métrique.** *Temps jusqu'à la première réponse correcte* pour un nouvel arrivant.
Humain ou agent, c'est la même mesure : l'onboarding d'une personne et celui d'un agent
sont le même problème, et un corpus de mémoire le résout une fois pour les deux.

---

## Corpus voisins

Une mémoire n'est jamais seule : elle cite un wiki, des runbooks, des commandes. Sans
déclaration, l'outil ne peut pas distinguer un lien externe valide d'un lien cassé.

```yaml
links:
  ignore_prefixes: ["/"]        # [[/deploy]] désigne une commande, pas une note
  external_roots:
    - ~/wiki                    # résolution par titre de fichier, à la Obsidian
```

Déclarer les voisins transforme le contrôle de liens en **contrôle d'intégrité
inter-corpus** : un renommage côté wiki casse le lien, et le lint le dit.

## Standards

`perimeter` ne réinvente rien de ce qui existe déjà :

- [`AGENTS.md`](https://agents.md) pour les directives — la mémoire ne remplace pas les instructions.
- [Agent Skills](https://code.claude.com/docs/en/skills) (`SKILL.md`) pour le procédural — une procédure s'exécute, elle ne se mémorise pas.
- [MCP](https://modelcontextprotocol.io) pour exposer la recherche à d'autres agents que celui qui a écrit.
- [ADR / MADR](https://adr.github.io) pour les décisions — le registre `decision` en reprend l'intention.
- [Diátaxis](https://diataxis.fr) pour la documentation humaine adjacente.
- JSON Schema pour le frontmatter, versionné et publié.

Et volontairement **pas** de base vectorielle. Sur du markdown atomique, un index et un
`grep` suffisent jusqu'à plusieurs milliers de notes. Le goulot d'étranglement n'est
jamais la récupération, c'est la curation : une recherche sémantique sur un corpus non
curé retrouve plus vite des informations fausses.

## Licence

MIT
