package ebnf

import (
	"io"
	"strings"
)

// CharacterGroupFunction check if rune is part of group
type CharacterGroupFunction func(r rune) bool

// CharacterGroup pattern, test membership of a group, for instance whitespace group
type CharacterGroup struct {
	BaseTransformer
	Group    CharacterGroupFunction
	Reversed bool
}

// NewCharacterGroup creates a new character group
func NewCharacterGroup(f CharacterGroupFunction, reversed bool) *CharacterGroup {
	return &CharacterGroup{
		Group:    f,
		Reversed: reversed,
	}
}

// NewCharacterGroupT creates a new character group with custom transform function
func NewCharacterGroupT(t TransformFunction, f CharacterGroupFunction, reversed bool) *CharacterGroup {
	return &CharacterGroup{
		BaseTransformer: BaseTransformer{
			T: t,
		},
		Group:    f,
		Reversed: reversed,
	}
}

// Match a character from a group
func (g *CharacterGroup) Match(r *Reader) (*MatchResult, error) {
	r.PushState()

	result := &MatchResult{Match: false}

	rn, err := r.Read()
	if err == io.EOF {
		r.RestoreState()
		return result, nil
	}

	if err != nil {
		r.RestoreState()
		return nil, err
	}

	if g.Reversed {
		result.Match = !g.Group(rn)
	} else {
		result.Match = g.Group(rn)
	}

	if result.Match {
		result.Result = r.String()
		g.Transform(result)
		r.PopState()
	} else {
		r.RestoreState()
	}

	return result, nil
}

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
