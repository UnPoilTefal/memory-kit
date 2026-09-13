---
name: user-equipe-plateforme
description: "L'equipe plateforme possede le corpus et arbitre les promotions de proposed vers canon"
metadata:
  type: user
  trust: canon
  owner: "@plateforme"
  modified: 2026-09-13
---

Le corpus appartient a l'equipe plateforme, qui relit les pull requests de memoire au
meme titre que celles de code.

Une note ecrite par un agent arrive en `trust: proposed`. Elle passe `canon` lorsqu'un
proprietaire de domaine l'a relue. Une note `personal` ne migre jamais vers `canon` sans
relecture : c'est precisement le chemin par lequel l'hypothese d'une personne deviendrait
la verite de l'equipe.
