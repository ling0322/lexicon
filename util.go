package lexicon

import (
	"log"
)

// assert check exp value, if exp == false then fatal with message
func assert(exp bool, message string) {
	if !exp {
		log.Fatal(message)
	}
}
