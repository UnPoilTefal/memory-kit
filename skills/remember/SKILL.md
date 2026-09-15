---
name: remember
description: Portillon d'écriture d'une note de mémoire — décider s'il faut écrire, puis écrire correctement. À invoquer avant d'ajouter quoi que ce soit à un corpus perimeter.
---

# remember

Ce skill décide **s'il faut écrire**, et seulement ensuite **comment**. La plupart
des invocations doivent se terminer par un refus : un corpus meurt bien plus
souvent par accumulation que par oubli.

## 1. Le portillon — trois questions, avant toute rédaction

Une seule réponse négative : **on n'écrit pas, et on dit pourquoi.**

1. **Est-ce non re-dérivable ?** Si un appel d'outil donne la réponse, ce n'est pas
   de la mémoire. Le nom d'un service, une version déclarée, la structure d'un
   dépôt : ça se redemande, ça ne se stocke pas. Ce qui reste est le **pourquoi**
   et les **pièges**.
2. **Est-ce non éphémère ?** Si c'est vrai cette semaine seulement, c'est une
   issue, pas une mémoire.
3. **Est-ce que ça comptera dans trois mois ?** Sinon le coût de relecture dépasse
   la valeur.

Ces trois questions ne se mécanisent pas. `perctl` ne les pose pas à votre place —
il ne juge que ce qui est mécanique.

**Le refus est un résultat.** Le dire explicitement, avec la question qui a échoué
et pourquoi, vaut mieux qu'écrire « au cas où ».

## 2. Vérifier contre le corpus, avant d'écrire

```bash
perctl draft brouillon.md          # ou : ... | perctl draft -
```

La commande **n'écrit jamais**. Elle rend :

| | |
|---|---|
| **collision** | une note porte déjà ce `name` — la compléter plutôt qu'en créer une seconde |
| **constats** | schéma, registre, description, longueur, secret |
| **voisinage** | « est-ce que ça existe déjà ? » — liste courte ordonnée, **à juger** |
| **preuve** | un bloc `verify` proposé quand la note énonce un fait testable |

⚠️ Le voisinage est un signal de **rang**, pas de classification. Un score de 0,15
sur un corpus entretenu peut désigner un vrai doublon ; 0,10 ne veut rien dire. Ne
jamais traiter le score comme un verdict : lire les termes communs et trancher.

Si un voisin dit déjà le fait, **compléter cette note** au lieu d'en ajouter une.
C'est le cas le plus fréquent, et le plus facile à manquer.

## 3. Écrire, si le portillon est passé

**Un fait par fichier.** C'est ce qui rend le dédoublonnage et la péremption
praticables. Deux faits, deux fichiers.

**La description énonce le fait, pas le sujet.** C'est elle — pas le corps — que
l'agent lit pour décider si la note est pertinente. Une mauvaise description rend
la note aussi inutile qu'une note absente de l'index.

- ✅ « Renovate retargète en interne la dernière version, ce qui fige une PR en pending »
- ❌ « notes sur Renovate »

**Le registre**, parmi les cinq : `user`, `feedback`, `reference`, `project`,
`decision`. Un `feedback` ou un `project` porte en plus **pourquoi** et **comment
l'appliquer**.

**Le bloc `verify`** quand le fait est testable. Une note factuelle sans preuve
rejouable se périme en silence — c'est le mode de panne que le corpus doit éviter.

⚠️ Une note qui énonce une **absence** appelle une sonde **inversée**. Et tester
l'existence d'un chemin ne prouve pas ce qu'il contient : choisir la commande qui
teste **la revendication de la note**, pas son voisinage.

## 4. Après l'écriture

Ajouter la ligne d'index — **l'index est le routeur du rappel**, une note absente
est écrite mais jamais lue — puis :

```bash
perctl lint
```

Selon `policy.index_hook`, l'accroche d'index se régénère (`derived`) ou se rédige
à la main (`authored`). En `authored`, ne jamais lancer `perctl index --sync` : il
refuse, précisément pour ne pas écraser ce travail.

## La contrainte qui tient tout

**Des propositions relues, jamais des écritures directes par un agent.** Une
mémoire qu'un agent modifie sans revue est le mécanisme exact par lequel une
hypothèse fausse devient vérité d'équipe en trois semaines.
