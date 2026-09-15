package harvest

import (
	"context"
	"time"
)

// contexte borne la lecture. Un depot enorme ne doit pas faire pendre la
// commande sans fin.
func contexte() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 60*time.Second)
}
