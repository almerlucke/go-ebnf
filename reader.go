package ebnf

import (
	"bufio"
	"io"
)

// Reader buffers runes to allow us to backtrack when the runes do not match a pattern
type Reader struct {
	reader              *bufio.Reader
	runeBuf             []rune
	runeBufPos          int
	runeBufPrevPosStack []int
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
