---
name: decision-pas-de-base-vectorielle
description: "Le corpus reste en markdown indexe, sans base vectorielle : le goulot est la curation, pas la recuperation"
metadata:
  type: decision
  trust: canon
  owner: "@plateforme"
  source: "ADR-0001"
  modified: 2026-09-13
  verify:
    - cmd: "! grep -qiE 'qdrant|weaviate|pinecone|chromadb' $(git rev-parse --show-toplevel)/go.mod"
      note: "aucune dependance de base vectorielle n'est entree dans le projet"
---

**Contexte.** La tentation, en voyant un corpus grossir, est d'ajouter une recherche
semantique.

**Decision.** On s'en tient a des notes atomiques, un index, et une recherche textuelle.

**Pourquoi.** Sur un corpus curate de quelques milliers de notes, l'index suffit. Le
facteur limitant n'est pas de retrouver une note, c'est qu'elle soit juste. Une recherche
semantique posee sur un corpus non curate ne fait que retrouver plus vite des
informations fausses — et elle deplace l'effort de la curation vers l'infrastructure,
c'est-a-dire du cote ou le probleme n'est pas.

**Consequence.** Le budget d'effort va aux regles de lint et aux preuves, pas au moteur
de recherche. A reconsiderer si le corpus depasse quelques milliers de notes **et** que
la curation est deja tenue.
