---
name: reference-index-est-le-routeur
description: "Une note absente de l'index est ecrite mais jamais lue : l'index est le routeur du rappel, pas une table des matieres de confort"
metadata:
  type: reference
  trust: canon
  owner: "@plateforme"
  modified: 2026-09-13
---

L'agent ne parcourt pas le corpus : il lit l'index, et decide a partir de lui quelles
notes ouvrir. Une note qui n'y figure pas a donc ete ecrite, relue et commitee — puis
n'existe plus du point de vue de l'agent.

Rien ne le signale sans outillage : la note est bien la, son contenu est correct, le
depot est propre. C'est le mode de perte le plus silencieux d'un corpus de memoire.

**Comment l'appliquer** : `perctl lint` traite ce cas en erreur, et `perctl index --fix`
ajoute les entrees manquantes. Voir aussi [[feedback-un-fait-par-note]].
