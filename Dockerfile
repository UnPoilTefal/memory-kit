# memctl ne lit que des fichiers et lance des commandes declarees : une image
# minimale suffit, et reduit la surface d'un outil qui tourne en CI.
FROM gcr.io/distroless/static-debian12

COPY memctl /usr/local/bin/memctl

ENTRYPOINT ["/usr/local/bin/memctl"]
