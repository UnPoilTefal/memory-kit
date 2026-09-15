# perctl ne lit que des fichiers et lance des commandes declarees : une image
# minimale suffit, et reduit la surface d'un outil qui tourne en CI.
FROM gcr.io/distroless/static-debian12

# Le contexte prepare par « dockers_v2 » range les binaires par plateforme
# (linux/amd64/perctl, linux/arm64/perctl) : une seule construction sert les
# deux architectures, a condition de passer par $TARGETPLATFORM.
ARG TARGETPLATFORM

COPY $TARGETPLATFORM/perctl /usr/local/bin/perctl

ENTRYPOINT ["/usr/local/bin/perctl"]
