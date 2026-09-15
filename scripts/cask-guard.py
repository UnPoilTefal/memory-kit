#!/usr/bin/env python3
"""Verifie qu'aucun appel propre a macOS ne traine sans garde d'OS dans le cask.

Le cask est publie pour quatre plateformes et n'etait teste que sur une. Un
appel a « xattr » sans garde a casse l'installation sous Linux : l'utilitaire
n'y existe pas, postflight avorte, et Homebrew purge tout ce qui venait d'etre
telecharge — sans message qui oriente vers la cause.

Ce controle coute une seconde et empeche la recidive de cette classe d'erreur.
Il ne remplace pas une vraie installation sur chaque plateforme.
"""
import re
import sys

APPELS_MACOS = ["xattr", "codesign", "spctl", "launchctl", "plutil", "defaults "]
PORTEE = 8  # lignes remontees pour chercher une garde ouverte


def non_gardes(contenu: str):
    lignes = contenu.split("\n")
    for i, ligne in enumerate(lignes):
        if not any(a in ligne for a in APPELS_MACOS):
            continue
        amont = "\n".join(lignes[max(0, i - PORTEE):i])
        if "OS.mac?" in amont or "on_macos" in amont:
            continue
        yield i + 1, ligne.strip()


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: cask-guard.py <cask.rb>", file=sys.stderr)
        return 2
    chemin = sys.argv[1]
    fautifs = list(non_gardes(open(chemin, encoding="utf-8").read()))
    if not fautifs:
        print(f"  ✓ {chemin} : aucun appel macOS sans garde d'OS")
        return 0
    for ligne, texte in fautifs:
        print(f"::error file={chemin},line={ligne}::appel propre a macOS sans garde OS.mac? — casse l'installation sur Linux : {texte}")
    return 1


if __name__ == "__main__":
    sys.exit(main())
