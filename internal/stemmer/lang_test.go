package stemmer

import (
	"fmt"
	"strings"
	"testing"

	"marrow/internal/testutil"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		query string
		want  string
	}{
		// English – basic stopwords and common terms
		{"the quick brown fox", "en"},
		{"how to use git", "en"},
		{"software configuration", "en"},
		{"Go Programming", "en"},
		{"a b c", "en"},
		{"123 456", "en"},

		// Spanish – stopwords and unique characters
		{"el libro", "es"},
		{"la casa", "es"},
		{"configuración", "es"},
		{"¿qué es esto?", "es"},
		{"señor García", "es"},
		{"cómo funciona esto", "es"},
		{"qué", "es"},
		{"ñ", "es"},
		{"¡hola!", "es"},
		{"más o menos", "es"},

		// Basque – digraphs and common words
		{"txistu", "eu"},
		{"etxe", "eu"},
		{"hitz", "eu"},
		{"Euskal Herria", "eu"},
		{"eta", "eu"},
		{"ez da", "eu"},
		{"nire etxea", "eu"},
		{"tx", "eu"},
		{"tz", "eu"},
		{"zer da hau", "eu"},
		{"Donostia kalean", "eu"},

		// Edge cases that previously mis-detected
		{"meta analysis", "en"},                // "eta" inside "meta" must not trigger Basque
		{"cats and dogs", "en"},                // "ts" in "cats" must not trigger Basque
		{"next", "en"},                         // no false Basque from tx/tz
		{"matrix", "en"},                       // no false Basque
		{"how to configure eta", "en"},         // English context overrides lone "eta"
		{"who is señor Garcia", "es"},          // ñ overrides English stopwords
		{"a", "en"},                            // ambiguous single-letter word, default to English
		{"el", "es"},                           // unambiguous Spanish

		// Mixed-context edge cases
		{"el famoso txistu", "eu"},             // lone Spanish article loses to strong Basque digraph
		{"el famoso txistu vasco español", "es"}, // several Spanish words overcome txistu
		{"Go Programming en español", "es"},    // Spanish words at end win
		{"Git eta GitHub artean", "eu"},        // Basque context with technical terms
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got := DetectLanguage(tt.query)
			if got != tt.want {
				t.Errorf("DetectLanguage(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestDetectLanguage_Approved(t *testing.T) {
	tests := []struct {
		query    string
		expected string
	}{
		// English
		{"the quick brown fox", "en"},
		{"how to use git", "en"},
		{"software configuration", "en"},
		{"Go Programming", "en"},
		{"a b c", "en"},
		{"123 456", "en"},
		{"meta analysis", "en"},
		{"cats and dogs", "en"},
		{"next", "en"},
		{"matrix", "en"},
		{"how to configure eta", "en"},
		{"a", "en"},
		{"i have nothing to declare", "en"},
		{"this is very important", "en"},
		{"the following public announcement", "en"},
		{"can you do that", "en"},
		{"well done", "en"},
		{"good bad new old", "en"},
		{"first last long great", "en"},

		// Spanish
		{"el libro", "es"},
		{"la casa", "es"},
		{"configuración", "es"},
		{"¿qué es esto?", "es"},
		{"señor García", "es"},
		{"cómo funciona esto", "es"},
		{"qué", "es"},
		{"ñ", "es"},
		{"¡hola!", "es"},
		{"más o menos", "es"},
		{"who is señor Garcia", "es"},
		{"el", "es"},
		{"Go Programming en español", "es"},
		{"el famoso txistu vasco español", "es"},
		{"la mejor casa", "es"},
		{"todos los días", "es"},
		{"aquí y ahora", "es"},
		{"bien hecho", "es"},
		{"porque sí", "es"},
		{"una persona importante", "es"},

		// Basque
		{"txistu", "eu"},
		{"etxe", "eu"},
		{"hitz", "eu"},
		{"Euskal Herria", "eu"},
		{"eta", "eu"},
		{"ez da", "eu"},
		{"nire etxea", "eu"},
		{"tx", "eu"},
		{"tz", "eu"},
		{"zer da hau", "eu"},
		{"Donostia kalean", "eu"},
		{"el famoso txistu", "eu"},
		{"Git eta GitHub artean", "eu"},
		{"hemen eta orain", "eu"},
		{"nire etxe ondoan", "eu"},
		{"euskal herriko kaleak", "eu"},
		{"bat bi gutxi guzti", "eu"},
		{"inoiz beti hemendik", "eu"},
		{"non nola noiz zergatik", "eu"},

		// Ambiguous / edge
		{"", "en"},
		{"x y z", "en"},
		{"123", "en"},
		{"el meta tx", "eu"},
		{"la matrix tz", "eu"},
	}

	var sb strings.Builder
	for _, tt := range tests {
		got := DetectLanguage(tt.query)
		status := "OK"
		if got != tt.expected {
			status = "FAIL"
		}
		fmt.Fprintf(&sb, "%-40s -> %s (want %s) [%s]\n", fmt.Sprintf("%q", tt.query), got, tt.expected, status)
	}
	testutil.VerifyApprovedString(t, sb.String())
}
