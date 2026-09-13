// Package schema embarque le JSON Schema des notes de memoire, pour que le
// fichier publie et la validation faite par l'outil ne puissent pas diverger.
package schema

import _ "embed"

//go:embed memory.schema.json
var Memory []byte

// URL est l'identifiant du schema, utilisable dans un editeur.
const URL = "https://github.com/UnPoilTefal/memory-kit/schema/memory.schema.json"
