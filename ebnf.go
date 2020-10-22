/*
Package ebnf allows to construct a set of EBNF type rules and provides a reader and struct/methods to
define, parse and validate any type of context-free grammar. ebfn_test.go shows
an example of how to describe a simple pascal like syntax as shown in
https://en.wikipedia.org/wiki/Extended_Backus%E2%80%93Naur_form. For each pattern
you can specify a transform function which can be used to transform the default
matched result output to a custom domain, for instance to construct language
source trees
*/
package ebnf

import (
	"io"
	"strings"
)

// MatchResult contains the result of a match
type MatchResult struct {
	Match    bool
	BeginPos *ReaderPos
	EndPos   *ReaderPos
	Result   interface{}
}

// Pattern to match
type Pattern interface {
	Match(r *Reader) (*MatchResult, error)
}

// TransformFunction for match result, this allows for custom transform of the result
type TransformFunction func(m *MatchResult, r *Reader) error

// Transformer for match result
type Transformer interface {
	Transform(m *MatchResult, r *Reader) error
}

// BaseTransformer implements transformer interface
type BaseTransformer struct {
	T TransformFunction
}

// Transform for base transformer
func (b *BaseTransformer) Transform(m *MatchResult, r *Reader) error {
	if b.T != nil {
		return b.T(m, r)
	}

	return nil
}

// TerminalString pattern
type TerminalString struct {
	BaseTransformer
	String string
}

// NewTerminalString creates a new terminal string
func NewTerminalString(s string, t TransformFunction) *TerminalString {
	return &TerminalString{
		BaseTransformer: BaseTransformer{
			T: t,
		},
		String: s,
	}
}

// Match a terminal string, MatchResult.Result will contain a string
func (s *TerminalString) Match(r *Reader) (*MatchResult, error) {
	beginPos := r.CurrentPosition()

	r.PushState()

	result := &MatchResult{Match: false}

	for _, rn1 := range []rune(s.String) {
		rn2, err := r.Read()
		if err != nil {
			r.RestoreState()

			if err == io.EOF {
				return result, nil
			}

			return nil, err
		}

		if rn1 != rn2 {
			r.RestoreState()
			return result, nil
		}
	}

	result.BeginPos = beginPos
	result.EndPos = r.CurrentPosition()
	result.Match = true
	result.Result = r.String()

	err := s.Transform(result, r)
	if err != nil {
		r.RestoreState()
		return nil, err
	}

	r.PopState()

	return result, nil
}

// CharacterGroupFunction check if rune is part of group
type CharacterGroupFunction func(r rune) bool

// CharacterGroup pattern, test membership of a group, for instance whitespace group
type CharacterGroup struct {
	BaseTransformer
	Group    CharacterGroupFunction
	Reversed bool
}

// NewCharacterGroup creates a new character group
func NewCharacterGroup(f CharacterGroupFunction, reversed bool, t TransformFunction) *CharacterGroup {
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
		err = g.Transform(result, r)
		if err != nil {
			r.RestoreState()
			return nil, err
		}

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

// Alternation pattern
type Alternation struct {
	BaseTransformer
	Patterns []Pattern
}

// NewAlternation creates a new alternation pattern
func NewAlternation(patterns []Pattern, t TransformFunction) *Alternation {
	return &Alternation{
		BaseTransformer: BaseTransformer{
			T: t,
		},
		Patterns: patterns,
	}
}

// Match alternation pattern, matches if one of the alternating patterns matches, returns the first matching pattern
func (a *Alternation) Match(r *Reader) (*MatchResult, error) {
	for _, p := range a.Patterns {
		finished := r.Finished()

		if finished {
			break
		}

		r.PushState()

		result, err := p.Match(r)
		if err != nil {
			r.RestoreState()
			return nil, err
		}

		if result.Match {
			err = a.Transform(result, r)
			if err != nil {
				r.RestoreState()
				return nil, err
			}

			r.PopState()

			return result, nil
		}

		r.RestoreState()
	}

	return &MatchResult{
		Match: false,
	}, nil
}

// Concatenation pattern
type Concatenation struct {
	BaseTransformer
	Patterns []Pattern
}

// NewConcatenation creates a new concatenation pattern
func NewConcatenation(patterns []Pattern, t TransformFunction) *Concatenation {
	return &Concatenation{
		BaseTransformer: BaseTransformer{
			T: t,
		},
		Patterns: patterns,
	}
}

// Match concatenation pattern, MatchResult.Result will contain []*MatchResult
func (c *Concatenation) Match(r *Reader) (*MatchResult, error) {
	beginPos := r.CurrentPosition()
	matches := []*MatchResult{}

	r.PushState()

	for _, p := range c.Patterns {
		result, err := p.Match(r)
		if err != nil {
			r.RestoreState()
			return nil, err
		}

		if !result.Match {
			r.RestoreState()
			return &MatchResult{
				Match: false,
			}, nil
		}

		matches = append(matches, result)
	}

	result := &MatchResult{
		BeginPos: beginPos,
		EndPos:   r.CurrentPosition(),
		Match:    true,
		Result:   matches,
	}

	err := c.Transform(result, r)
	if err != nil {
		r.RestoreState()
		return nil, err
	}

	r.PopState()

	return result, nil
}

// Repetition pattern
type Repetition struct {
	BaseTransformer
	Pattern Pattern
	Min     int
	Max     int
}

// NewRepetition creates a new repetition pattern
func NewRepetition(p Pattern, min int, max int, t TransformFunction) *Repetition {
	return &Repetition{
		BaseTransformer: BaseTransformer{
			T: t,
		},
		Pattern: p,
		Min:     min,
		Max:     max,
	}
}

// NewOptional creates a new repetition pattern with 0-1
func NewOptional(p Pattern, t TransformFunction) *Repetition {
	return &Repetition{
		BaseTransformer: BaseTransformer{
			T: t,
		},
		Pattern: p,
		Min:     0,
		Max:     1,
	}
}

// Match repetition pattern, MatchResult.Result will contain []*MatchResult
func (rep *Repetition) Match(r *Reader) (*MatchResult, error) {
	beginPos := r.CurrentPosition()
	matches := []*MatchResult{}

	r.PushState()

	for {
		finished := r.Finished()
		if finished {
			break
		}

		result, err := rep.Pattern.Match(r)
		if err != nil {
			r.RestoreState()
			return nil, err
		}

		if result.Match {
			matches = append(matches, result)

			if rep.Max != 0 && len(matches) == rep.Max {
				break
			}
		} else {
			break
		}
	}

	if len(matches) < rep.Min {
		r.RestoreState()
		return &MatchResult{
			Match: false,
		}, nil
	}

	result := &MatchResult{
		BeginPos: beginPos,
		EndPos:   r.CurrentPosition(),
		Match:    true,
		Result:   matches,
	}

	err := rep.Transform(result, r)
	if err != nil {
		r.RestoreState()
		return nil, err
	}

	r.PopState()

	return result, nil
}

// Exception pattern
type Exception struct {
	BaseTransformer
	MustMatch Pattern
	Except    Pattern
}

// NewException creates a new exception
func NewException(mustMatch Pattern, except Pattern, t TransformFunction) *Exception {
	return &Exception{
		BaseTransformer: BaseTransformer{
			T: t,
		},
		MustMatch: mustMatch,
		Except:    except,
	}
}

// Match exception pattern, returns the MustMatch match result
func (e *Exception) Match(r *Reader) (result *MatchResult, err error) {
	r.PushState()

	result, err = e.Except.Match(r)
	if err != nil {
		return
	}

	if result.Match {
		r.RestoreState()
		result.Match = false
		result.Result = nil
		return
	}

	r.PopState()

	result, err = e.MustMatch.Match(r)
	if err != nil {
		return
	}

	err = e.Transform(result, r)

	return
}

// EOF pattern
type EOF struct {
	BaseTransformer
}

// NewEOF creates a new end of file
func NewEOF(t TransformFunction) *EOF {
	return &EOF{
		BaseTransformer: BaseTransformer{
			T: t,
		},
	}
}

// Match end of file pattern
func (e *EOF) Match(r *Reader) (result *MatchResult, err error) {
	match := r.Finished()

	result = &MatchResult{
		Match: match,
	}

	err = e.Transform(result, r)

	return
}
