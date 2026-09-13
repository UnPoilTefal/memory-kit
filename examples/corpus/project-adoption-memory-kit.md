---
name: project-adoption-memory-kit
description: "L'adoption commence par le lint en CI seul ; les preuves ne s'ajoutent qu'ensuite, en partant des faits qui ont deja coute un incident"
metadata:
  type: project
  trust: canon
  owner: "@plateforme"
  modified: 2026-09-13
---

**Etat.** Sequence d'adoption retenue pour une equipe qui part d'un corpus existant.

1. `memctl lint` en CI, sans `--strict`. Il ne casse que sur les erreurs structurelles,
   ce qui rend la premiere pull request acceptable.
2. Resorber les orphelins d'index. C'est le gain le plus immediat et le plus visible :
   des notes deja ecrites redeviennent accessibles.
3. Ajouter `owner` et `trust`, activer `require_owner`.
4. Seulement ensuite, les preuves — en commencant par les faits qui ont deja cause un
   incident. Le taux de couverture monte lentement, c'est normal.

**Pourquoi cet ordre.** Commencer par les preuves donne un corpus verifie dont la
structure ne tient pas. L'inverse ne marche pas : on ne peut pas verifier ce qu'on ne
sait pas retrouver.
