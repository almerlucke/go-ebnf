package main

import (
	"bufio"
	"io"
	"log"
	"strings"
	"unicode"
)

/*
terminal symbols -> "..." | '...', special sequence (for instance ? whitespace ?)
production rule -> alternation, concatenation, optional, repetition, grouping, exception
non terminals
*/

// Reader buffers runes read until they are consumed, this allows us to read runes ahead
// and go back (reset) when the runes do not match a pattern
type Reader struct {
	reader              *bufio.Reader
	runeBuf             []rune
	runeBufPos          int
	runeBufPrevPosStack []int
}

// MatchResult contains the result of a match
type MatchResult struct {
	Match  bool
	Result interface{}
}

// Pattern to match, can use peak and read from reader
type Pattern interface {
	Match(r *Reader) (*MatchResult, error)
}

// TerminalString pattern
type TerminalString string

// TerminalCharacterGroup pattern, for instance whitespace group
type TerminalCharacterGroup func(r rune) bool

// Alternation pattern
type Alternation []Pattern

// Repetition pattern
type Repetition struct {
	Min     int
	Max     int
	Pattern Pattern
}

// Exception pattern
type Exception struct {
	Must    Pattern
	MustNot Pattern
}

type PatternType int

const (
	TerminalSymbolPattern PatternType = iota
	TerminalCharacterGroupPattern
	AlternationPattern
	ConcatenationPattern
	OptionalPattern
	RepetitionPattern
	ExceptionPattern
	RulePattern
)

type Element struct {
	Pattern Pattern
	Type    Pattern
}

// NewReader creates a new reader
func NewReader(r io.Reader) *Reader {
	return &Reader{
		reader:              bufio.NewReader(r),
		runeBuf:             []rune{},
		runeBufPrevPosStack: []int{0},
	}
}

// SavePos pushes the current buffer position on the stack
func (r *Reader) SavePos() {
	r.runeBufPrevPosStack = append(r.runeBufPrevPosStack, r.runeBufPos)
}

// RestorePos pops and restores the buffer position to the last pushed buffer position from the stack
func (r *Reader) RestorePos() {
	l := len(r.runeBufPrevPosStack) - 1
	r.runeBufPos, r.runeBufPrevPosStack = r.runeBufPrevPosStack[l], r.runeBufPrevPosStack[:l]
}

// PopPos pops the last pushed buffer position from the stack without restoring
func (r *Reader) PopPos() {
	l := len(r.runeBufPrevPosStack) - 1
	r.runeBufPrevPosStack = r.runeBufPrevPosStack[:l]
}

// String gets the current buffer content between the previous pos and the current pos as string
func (r *Reader) String() string {
	prevPos := r.runeBufPrevPosStack[len(r.runeBufPrevPosStack)-1]
	return string(r.runeBuf[prevPos:r.runeBufPos])
}

// Consume from the buffer what is read so far
func (r *Reader) Consume() {
	if r.runeBufPos < len(r.runeBuf) {
		r.runeBuf = r.runeBuf[r.runeBufPos:]
	} else {
		r.runeBuf = []rune{}
	}

	r.runeBufPos = 0
	r.runeBufPrevPosStack = []int{0}
}

// Finished returns true if EOF is reached
func (r *Reader) Finished() (bool, error) {
	_, err := r.Peak()
	if err != nil {
		if err == io.EOF {
			return true, nil
		}
		return false, err
	}

	return false, nil
}

// Peak returns the next rune without advancing the read position
func (r *Reader) Peak() (rn rune, err error) {
	if r.runeBufPos < len(r.runeBuf) {
		rn = r.runeBuf[r.runeBufPos]
	} else {
		rn, _, err = r.reader.ReadRune()
		if err == nil {
			err = r.reader.UnreadRune()
		}
	}

	return
}

// Read returns the next rune and advances the read position
func (r *Reader) Read() (rn rune, err error) {
	if r.runeBufPos < len(r.runeBuf) {
		rn = r.runeBuf[r.runeBufPos]
		r.runeBufPos++
	} else {
		rn, _, err = r.reader.ReadRune()
		if err == nil {
			r.runeBuf = append(r.runeBuf, rn)
			r.runeBufPos++
		}
	}

	return
}

// Match a terminal string
func (s TerminalString) Match(r *Reader) (*MatchResult, error) {
	r.SavePos()

	result := &MatchResult{
		Match: false,
	}
	runes := []rune(s)

	for _, rn1 := range runes {
		rn2, err := r.Read()
		if err != nil {
			r.RestorePos()

			if err == io.EOF {
				return result, nil
			}

			return nil, err
		}

		if rn1 != rn2 {
			r.RestorePos()
			return result, nil
		}
	}

	result.Match = true
	result.Result = r.String()

	r.PopPos()

	return result, nil
}

// Match a character from a group
func (g TerminalCharacterGroup) Match(r *Reader) (*MatchResult, error) {
	r.SavePos()

	result := &MatchResult{
		Match: false,
	}

	rn, err := r.Read()
	if err == io.EOF {
		r.RestorePos()
		return result, nil
	}

	if err != nil {
		r.RestorePos()
		return nil, err
	}

	result.Match = g(rn)
	if result.Match {
		result.Result = r.String()
		r.PopPos()
	} else {
		r.RestorePos()
	}

	return result, nil
}

// Match alternation pattern, matches if one of the alternating patterns matches
func (a Alternation) Match(r *Reader) (*MatchResult, error) {
	for _, p := range a {
		finished, err := r.Finished()
		if err != nil {
			return nil, err
		}

		if finished {
			break
		}

		r.SavePos()

		result, err := p.Match(r)
		if err != nil {
			return nil, err
		}

		if result.Match {
			r.PopPos()
			return result, nil
		}

		r.RestorePos()
	}

	return &MatchResult{Match: false}, nil
}

// Match exception pattern
func (e *Exception) Match(r *Reader) (result *MatchResult, err error) {
	result, err = e.MustNot.Match(r)
	if err != nil {
		return
	}

	if result.Match {
		result.Match = false
		result.Result = nil
		return
	}

	result, err = e.Must.Match(r)

	return
}

// Match repetition pattern
func (rep *Repetition) Match(r *Reader) (*MatchResult, error) {
	numMatches := 0
	matches := []*MatchResult{}

	r.SavePos()

	for {
		finished, err := r.Finished()
		if err != nil {
			return nil, err
		}

		if finished {
			break
		}

		result, err := rep.Pattern.Match(r)
		if err != nil {
			r.RestorePos()
			return nil, err
		}

		if result.Match {
			numMatches++
			matches = append(matches, result)

			if rep.Max != 0 && numMatches == rep.Max {
				break
			}
		} else {
			break
		}
	}

	if numMatches < rep.Min {
		r.RestorePos()
		return &MatchResult{
			Match: false,
		}, nil
	}

	r.PopPos()

	return &MatchResult{
		Match:  true,
		Result: matches,
	}, nil
}

func main() {
	reader := NewReader(strings.NewReader("123 this is a test"))

	repetition := &Repetition{
		Min:     1,
		Max:     0,
		Pattern: TerminalCharacterGroup(unicode.IsLetter),
	}

	terminal1 := TerminalString("123")
	terminal2 := TerminalString("this")
	alternation := Alternation{terminal1, terminal2}

	whitespace := TerminalCharacterGroup(unicode.IsSpace)

	result, err := repetition.Match(reader)
	if err != nil {
		log.Fatalf("err %v\n", err)
	}

	if result.Match {
		log.Printf("matched %v\n", result.Result)
	} else {
		log.Printf("no match\n")
	}

	result, err = whitespace.Match(reader)
	if err != nil {
		log.Fatalf("err %v\n", err)
	}

	if result.Match {
		log.Printf("matched %v\n", result.Result)
	} else {
		log.Printf("no match\n")
	}

	result, err = alternation.Match(reader)
	if err != nil {
		log.Fatalf("err %v\n", err)
	}

	if result.Match {
		log.Printf("matched %v\n", result.Result)
	} else {
		log.Printf("no match\n")
	}
}
