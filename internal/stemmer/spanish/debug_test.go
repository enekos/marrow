package spanish

import (
	"fmt"
	"testing"
)

func TestDebugAllFailing(t *testing.T) {
	// Generate expected values for failing words
	cases := []struct{ word string; stemStop bool }{
		{"mujeres", true},
		{"día", true}, {"dias", true},
		{"eres", true},
		{"incapaz", true}, {"capaz", true},
		{"día", true},
		{"amándola", true}, {"amándolas", true},
		{"violencia", true}, {"violencias", true},
		{"distancia", true}, {"distancias", true},
		{"revolución", true}, {"revoluciones", true},
		{"tendencia", true}, {"tendencias", true},
		{"experiencia", true}, {"experiencias", true},
		{"rentable", true}, {"rentables", true},
		{"probable", true}, {"probables", true},
		{"vegetal", true}, {"vegetales", true},
		{"animal", true}, {"animales", true},
		{"mortal", true}, {"mortales", true},
		{"nacional", true}, {"nacionales", true},
		{"original", true}, {"originales", true},
		{"general", true}, {"generales", true},
		{"formal", true}, {"formales", true},
		{"normal", true}, {"normales", true},
		{"fatal", true}, {"fatales", true},
		{"vertical", true}, {"verticales", true},
		{"total", true}, {"totales", true},
		{"local", true}, {"locales", true},
		{"ideal", true}, {"ideales", true},
		{"real", true}, {"reales", true},
		{"personal", true}, {"personales", true},
		{"natural", true}, {"naturales", true},
	}
	for _, c := range cases {
		fmt.Printf("%q (%v): %q\n", c.word, c.stemStop, Stem(c.word, c.stemStop))
	}
}
