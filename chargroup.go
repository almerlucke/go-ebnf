package ebnf

import "strings"

// NewCharacterGroupRangeFunction returns a function which can be used as a CharacterGroupFunction
// with a low and high range for the input rune to match
func NewCharacterGroupRangeFunction(low rune, high rune) CharacterGroupFunction {
	return func(r rune) bool {
		return r >= low && r <= high
	}
}

// NewCharacterGroupEnumFunction returns a function which can be used as a CharacterGroupFunction
// with a string enum which can be tested for the input rune membership
func NewCharacterGroupEnumFunction(enum string) CharacterGroupFunction {
	return func(r rune) bool {
		return strings.ContainsRune(enum, r)
	}
}
