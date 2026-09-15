package corpus

import "testing"

// Declarer un bloc corpus sans clé index desactivait l'index : le champ etait
// recopie tel quel, et une chaine vide signifie « pas d'index ». Absent et
// vide doivent se distinguer — le premier prend le defaut, le second dit
// explicitement qu'il n'y en a pas.
func TestIndexAbsentPrendLeDefautIndexVideLeDesactive(t *testing.T) {
	vide := ""
	memory := "MEMORY.md"
	autre := "INDEX.md"

	for _, cas := range []struct {
		nom     string
		valeur  *string
		attendu string
	}{
		{"absent", nil, "MEMORY.md"},
		{"vide", &vide, ""},
		{"memory", &memory, "MEMORY.md"},
		{"autre", &autre, "INDEX.md"},
	} {
		p := politiqueAvecIndex(cas.valeur)
		if got := ConfigFromPolicy(p).Corpus.Index; got != cas.attendu {
			t.Errorf("index %s : attendu %q, obtenu %q", cas.nom, cas.attendu, got)
		}
	}
}
