package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"marrow/internal/stemmer"
)

func main() {
	lang := flag.String("lang", "en", "Language code (en, es, eu)")
	flag.Parse()

	supported := map[string]bool{"en": true, "es": true, "eu": true}
	if !supported[*lang] {
		fmt.Fprintf(os.Stderr, "unsupported language: %q (supported: en, es, eu)\n", *lang)
		os.Exit(1)
	}

	args := flag.Args()
	if len(args) > 0 {
		for _, arg := range args {
			printStemmed(arg, *lang)
		}
		return
	}

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		printStemmed(scanner.Text(), *lang)
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(1)
	}
}

func printStemmed(text, lang string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	tokens := stemmer.Tokenize(text)
	filtered := stemmer.FilterStopWords(tokens, lang)
	stemmed := stemmer.StemText(text, lang)
	fmt.Printf("tokens:  %v\n", tokens)
	fmt.Printf("filtered:%v\n", filtered)
	fmt.Printf("stemmed: %s\n\n", stemmed)
}
