# memctl ne lit que des fichiers et lance des commandes declarees : une image
# minimale suffit, et reduit la surface d'un outil qui tourne en CI.
FROM gcr.io/distroless/static-debian12

# Le contexte prepare par « dockers_v2 » range les binaires par plateforme
# (linux/amd64/memctl, linux/arm64/memctl) : une seule construction sert les
# deux architectures, a condition de passer par $TARGETPLATFORM.
ARG TARGETPLATFORM

COPY $TARGETPLATFORM/memctl /usr/local/bin/memctl

ENTRYPOINT ["/usr/local/bin/memctl"]
