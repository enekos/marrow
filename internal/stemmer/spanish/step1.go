package spanish

import (
	"github.com/enekos/marrow/internal/stemmer/snowballword"
)

var step1Suffixes = [][]rune{
	[]rune("amientos"),
	[]rune("imientos"),
	[]rune("aciones"),
	[]rune("amiento"),
	[]rune("imiento"),
	[]rune("uciones"),
	[]rune("logías"),
	[]rune("idades"),
	[]rune("encias"),
	[]rune("ancias"),
	[]rune("amente"),
	[]rune("adores"),
	[]rune("adoras"),
	[]rune("ución"),
	[]rune("mente"),
	[]rune("logía"),
	[]rune("istas"),
	[]rune("ismos"),
	[]rune("ibles"),
	[]rune("encia"),
	[]rune("anzas"),
	[]rune("antes"),
	[]rune("ancia"),
	[]rune("adora"),
	[]rune("ación"),
	[]rune("ables"),
	[]rune("osos"),
	[]rune("osas"),
	[]rune("ivos"),
	[]rune("ivas"),
	[]rune("ista"),
	[]rune("ismo"),
	[]rune("idad"),
	[]rune("icos"),
	[]rune("icas"),
	[]rune("ible"),
	[]rune("anza"),
	[]rune("ante"),
	[]rune("ador"),
	[]rune("able"),
	[]rune("oso"),
	[]rune("osa"),
	[]rune("ivo"),
	[]rune("iva"),
	[]rune("ico"),
	[]rune("ica"),
}

var (
	runeIC  = []rune("ic")
	runeOS  = []rune("os")
	runeAD  = []rune("ad")
	runeANT = []rune("ant")
	runeABL = []rune("abl")
	runeIBL = []rune("ibl")
	runeABIL= []rune("abil")
	runeIV  = []rune("iv")
	runeAT  = []rune("at")
)

var step1Replacements = map[string][]rune{
	"logía":  []rune("log"),
	"logías": []rune("log"),
	"ución":  []rune("u"),
	"uciones":[]rune("u"),
	"encia":  []rune("ente"),
	"encias": []rune("ente"),
}

// Step 1 is the removal of standard suffixes
//
func step1(word *snowballword.SnowballWord) bool {

	// Possible suffixes, longest first
	suffixRunes := word.FirstSuffixRunes(step1Suffixes...)

	isInR1 := (word.R1start <= len(word.RS)-len(suffixRunes))
	isInR2 := (word.R2start <= len(word.RS)-len(suffixRunes))

	// Deal with special cases first.  All of these will
	// return if they are hit.
	//
	switch string(suffixRunes) {
	case "":
		// Nothing to do
		return false

	case "amente":
		if isInR1 {
			// Delete if in R1
			word.RemoveLastNRunes(len(suffixRunes))

			// if preceded by iv, delete if in R2 (and if further preceded by at,
			// delete if in R2), otherwise,
			// if preceded by os, ic or ad, delete if in R2
			newSuffixRunes := word.RemoveFirstSuffixIfInRunes(word.R2start, runeIV, runeOS, runeIC, runeAD)
			if newSuffixRunes != nil && string(newSuffixRunes) == "iv" {
				word.RemoveFirstSuffixIfInRunes(word.R2start, runeAT)
			}
			return true
		}
		return false
	}

	// All the following cases require the found suffix
	// to be in R2.
	if !isInR2 {
		return false
	}

	// Compound replacement cases.  All these cases return
	// if they are hit.
	//
	switch string(suffixRunes) {
	case "adora", "ador", "ación", "adoras", "adores", "aciones", "ante", "antes", "ancia", "ancias":
		word.RemoveLastNRunes(len(suffixRunes))
		word.RemoveFirstSuffixIfInRunes(word.R2start, runeIC)
		return true
	case "mente":
		word.RemoveLastNRunes(len(suffixRunes))
		word.RemoveFirstSuffixIfInRunes(word.R2start, runeANT, runeABL, runeIBL)
		return true
	case "idad", "idades":
		word.RemoveLastNRunes(len(suffixRunes))
		word.RemoveFirstSuffixIfInRunes(word.R2start, runeABIL, runeIC, runeIV)
		return true
	case "iva", "ivo", "ivas", "ivos":
		word.RemoveLastNRunes(len(suffixRunes))
		word.RemoveFirstSuffixIfInRunes(word.R2start, runeAT)
		return true
	}

	// Simple replacement & deletion cases are all that remain.
	//
	repl := step1Replacements[string(suffixRunes)]
	if repl != nil {
		word.ReplaceSuffixRunes(suffixRunes, repl, true)
		return true
	}

	switch string(suffixRunes) {
	case "anza", "anzas", "ico", "ica", "icos", "icas",
		"ismo", "ismos", "able", "ables", "ible", "ibles",
		"ista", "istas", "oso", "osa", "osos", "osas",
		"amiento", "amientos", "imiento", "imientos":
		word.RemoveLastNRunes(len(suffixRunes))
		return true
	}

	return false
}
