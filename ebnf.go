/*
Package ebnf allows to construct a set of EBNF type pattern matcher and provides a reader
and struct/methods to define, parse and validate any type of context-free grammar. ebfn_test.go shows
an example of how to describe a simple pascal like syntax as shown in
https://en.wikipedia.org/wiki/Extended_Backus%E2%80%93Naur_form. For each pattern
you can specify a transform function which can be used to transform the default
matched result output to a custom domain, for instance to construct language
source trees
*/
package ebnf

import (
	"fmt"
	"io"
	"strings"
)

// MatchResult contains the result of a match
type MatchResult struct {
	Match        bool
	PartialMatch bool
	BeginPos     *ReaderPos
	EndPos       *ReaderPos
	Result       interface{}
	Error        error
	Failed       *MatchResult
}

// RangeString returns the range of the match as a string
func (m *MatchResult) RangeString() string {
	bl := m.BeginPos.linePos + 1
	bp := m.BeginPos.relativeCharPos + 1
	el := m.EndPos.linePos + 1
	ep := m.EndPos.relativeCharPos + 1
	return fmt.Sprintf("> line %d, pos %d --- line %d, pos %d <", bl, bp, el, ep)
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
	result.BeginPos = beginPos

	for _, rn1 := range []rune(s.String) {
		rn2, err := r.Read()
		if err != nil {
			if err == io.EOF {
				result.EndPos = r.CurrentPosition()

				err = s.Transform(result, r)
				if err != nil {
					return nil, err
				}

				r.RestoreState()

				return result, nil
			}

			return nil, err
		}

		if rn1 != rn2 {
			result.EndPos = r.CurrentPosition()

			err = s.Transform(result, r)
			if err != nil {
				return nil, err
			}

			r.RestoreState()

			return result, nil
		}
	}

	result.Match = true
	result.EndPos = r.CurrentPosition()
	result.Result = r.String()

	err := s.Transform(result, r)
	if err != nil {
		return nil, err
	}

	r.PopState()

	return result, nil
}

// CharacterGroupFunction check if rune is part of group
type CharacterGroupFunction func(r rune) bool

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

// NewCharacterEnum creates a new character enum group
func NewCharacterEnum(enum string, reversed bool, t TransformFunction) *CharacterGroup {
	return NewCharacterGroup(NewCharacterGroupEnumFunction(enum), reversed, t)
}

// NewCharacterRange creates a new character range group
func NewCharacterRange(low rune, high rune, reversed bool, t TransformFunction) *CharacterGroup {
	return NewCharacterGroup(NewCharacterGroupRangeFunction(low, high), reversed, t)
}

// Match a character from a group
func (g *CharacterGroup) Match(r *Reader) (*MatchResult, error) {
	beginPos := r.CurrentPosition()
	r.PushState()

	result := &MatchResult{Match: false}
	result.BeginPos = beginPos

	rn, err := r.Read()
	if err == io.EOF {
		result.EndPos = r.CurrentPosition()

		err = g.Transform(result, r)
		if err != nil {
			return nil, err
		}

		r.RestoreState()

		return result, nil
	}

	if err != nil {
		return nil, err
	}

	if g.Reversed {
		result.Match = !g.Group(rn)
	} else {
		result.Match = g.Group(rn)
	}

	if result.Match {
		result.Result = r.String()
		result.EndPos = r.CurrentPosition()

		err = g.Transform(result, r)
		if err != nil {
			return nil, err
		}

		r.PopState()
	} else {
		r.RestoreState()
	}

	return result, nil
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
	beginPos := r.CurrentPosition()
	var partialMatchResult *MatchResult = nil

	for _, p := range a.Patterns {
		finished := r.Finished()

		if finished {
			break
		}

		result, err := p.Match(r)
		if err != nil {
			return nil, err
		}

		if result.Match {
			err = a.Transform(result, r)
			if err != nil {
				return nil, err
			}

			return result, nil
		} else if result.PartialMatch {
			partialMatchResult = result
		}
	}

	result := &MatchResult{
		BeginPos: beginPos,
		EndPos:   beginPos,
		Match:    false,
		Failed:   partialMatchResult,
	}

	err := a.Transform(result, r)
	if err != nil {
		return nil, err
	}

	return result, nil
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

// Match concatenation pattern, MatchResult.Result will contain []*MatchResult if matched,
// otherwise MatchResult.Result will contain the failed match
func (c *Concatenation) Match(r *Reader) (*MatchResult, error) {
	beginPos := r.CurrentPosition()
	matches := []*MatchResult{}
	partialMatch := false

	r.PushState()

	for _, p := range c.Patterns {
		result, err := p.Match(r)
		if err != nil {
			return nil, err
		}

		if !result.Match {
			result = &MatchResult{
				BeginPos:     beginPos,
				EndPos:       r.CurrentPosition(),
				Match:        false,
				PartialMatch: partialMatch,
				Failed:       result, // Store unmatched result as result of the failed concatenation
			}

			err := c.Transform(result, r)
			if err != nil {
				return nil, err
			}

			r.RestoreState()

			return result, nil
		}

		partialMatch = true

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

// NewAny creates a new repetition pattern with 0-0
func NewAny(p Pattern, t TransformFunction) *Repetition {
	return &Repetition{
		BaseTransformer: BaseTransformer{
			T: t,
		},
		Pattern: p,
		Min:     0,
		Max:     0,
	}
}

// Match repetition pattern, MatchResult.Result will contain []*MatchResult
func (rep *Repetition) Match(r *Reader) (*MatchResult, error) {
	beginPos := r.CurrentPosition()
	matches := []*MatchResult{}

	var result *MatchResult
	var err error

	r.PushState()

	for {
		finished := r.Finished()
		if finished {
			break
		}

		result, err = rep.Pattern.Match(r)
		if err != nil {
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
		failedResult := result
		if result.Match {
			failedResult = nil
		}

		result = &MatchResult{
			Error:    fmt.Errorf("expected minimum of %d repetitions", rep.Min),
			BeginPos: beginPos,
			EndPos:   r.CurrentPosition(),
			Match:    false,
			Failed:   failedResult,
		}

		err = rep.Transform(result, r)
		if err != nil {
			return nil, err
		}

		r.RestoreState()

		return result, nil
	}

	result = &MatchResult{
		BeginPos: beginPos,
		EndPos:   r.CurrentPosition(),
		Match:    true,
		Result:   matches,
	}

	err = rep.Transform(result, r)
	if err != nil {
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
func (e *Exception) Match(r *Reader) (*MatchResult, error) {
	r.PushState()

	result, err := e.Except.Match(r)
	if err != nil {
		return nil, err
	}

	if result.Match {
		result.Match = false
		result.Failed = result

		err = e.Transform(result, r)
		if err != nil {
			return nil, err
		}

		r.RestoreState()

		return result, nil
	}

	r.PopState()

	result, err = e.MustMatch.Match(r)
	if err != nil {
		return nil, err
	}

	err = e.Transform(result, r)

	return result, err
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
