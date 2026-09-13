---
name: feedback-un-fait-par-note
description: "Une note porte un seul fait : au-dela, elle ne peut plus etre ni dedoublonnee ni perimee proprement"
metadata:
  type: feedback
  trust: canon
  owner: "@plateforme"
  modified: 2026-09-13
---

**Pourquoi.** Une note qui porte cinq faits se perime par morceaux. On ne peut ni la
supprimer (quatre faits sont encore vrais), ni la garder (le cinquieme est faux), ni la
dedoublonner contre une autre note qui n'en recouvre qu'une partie. Elle devient
intouchable, et sa partie fausse survit indefiniment.

**Comment l'appliquer.** Au moment d'ecrire, si la description demande deux phrases, c'est
qu'il y a deux notes. `memctl lint` avertit au-dela du budget de mots configure, mais le
budget n'est qu'un filet : la vraie regle est celle de la description.
