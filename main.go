package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	//"github.com/pkg/profile"
)

var DEBUG = false

func _debug(msg string, args ...any) {
	log.Printf(msg, args...)
}

func _debugln(args ...any) {
	if !DEBUG {
		return
	}

	log.Println(args...)
}

var debug = func(msg string, args ...any) {}
var debugln = func(args ...any) {}

type jsonReader struct {
	read Vector[byte]

	// Reused memory
	expectStringCache     Vector[byte]
	eatValueCache         Vector[byte]
	expectIdentifierCache Vector[byte]
	tryNumberHint         Vector[byte]
	tryNumberNumber       Vector[byte]
}

func (jr *jsonReader) reset() {
	jr.read.Reset()
}

func (jr *jsonReader) readByteRaw(r *bufio.Reader) (byte, error) {
	c, err := r.ReadByte()
	if err != nil {
		return byte(0), err
	}

	jr.read.Append(c)

	return c, nil
}

func (jr *jsonReader) readByte(r *bufio.Reader) (byte, error) {
	return jr.readByteRaw(r)
}

func (jr *jsonReader) discard(r *bufio.Reader) {
	r.Discard(1)
}

func (jr *jsonReader) peek(r *bufio.Reader) (byte, error) {
	bs, err := r.Peek(1)
	if err != nil {
		return byte(0), err
	}
	return bs[0], err
}

func (jr *jsonReader) eatWhitespace(r *bufio.Reader) error {
	for {
		b, err := jr.peek(r)
		if err != nil {
			return err
		}

		isWhitespace := b == ' ' ||
			b == '\n' ||
			b == '\t' ||
			b == '\r'
		if !isWhitespace {
			return nil
		}

		jr.discard(r)
	}
}

func (jr *jsonReader) expectString(r *bufio.Reader) ([]byte, error) {
	jr.expectStringCache.Reset()

	err := jr.eatWhitespace(r)
	if err != nil {
		return nil, err
	}

	b, err := jr.readByte(r)
	if err != nil {
		return nil, err
	}

	if b != '"' {
		return nil, fmt.Errorf("Expected double quote to start string, got: '%s'", string(b))
	}

	var prev byte
	for {
		b, err := jr.readByte(r)
		if err != nil {
			return nil, err
		}

		if b == '\\' && prev == '\\' {
			// Just skip
			prev = byte(0)
			continue
		} else if b == '"' {
			// Overwrite the escaped double quote
			if prev == '\\' {
				jr.expectStringCache.Insert(jr.expectStringCache.Index()-1, '"')
			} else {
				// Otherwise it's the actual end
				break
			}
		}

		jr.expectStringCache.Append(b)
		prev = b
	}

	return jr.expectStringCache.List(), nil
}

func (jr *jsonReader) expectIdentifier(r *bufio.Reader, ident []byte, value any) (any, error) {
	jr.expectIdentifierCache.Reset()

	for i := 0; i < len(ident); i++ {
		b, err := jr.peek(r)
		if err != nil {
			return nil, err
		}

		jr.expectIdentifierCache.Append(b)

		jr.discard(r)
	}

	if bytes.Equal(jr.expectIdentifierCache.List(), ident) {
		return value, nil
	}

	return nil, fmt.Errorf("Unknown value: '%s'", string(jr.expectIdentifierCache.List()))
}

func (jr *jsonReader) tryNumber(r *bufio.Reader) (bool, any, error) {
	jr.tryNumberHint.Reset()
	jr.tryNumberNumber.Reset()

	for {
		// TODO: replace this with bulk peek a la eatValue
		c, err := jr.peek(r)
		if err != nil {
			return false, nil, err
		}

		jr.tryNumberHint.Append(c)

		isNumberCharacter := (c >= '0' && c <= '9') || c == 'e' || c == '-'
		if !isNumberCharacter {
			break
		}

		jr.tryNumberNumber.Append(c)

		jr.discard(r)
	}

	if jr.tryNumberNumber.Index() == 0 {
		return false, nil, nil
	}

	var n float64
	err := json.Unmarshal(jr.tryNumberNumber.List(), &n)
	return true, n, err
}

func (jr *jsonReader) eatValue(r *bufio.Reader) error {
	jr.eatValueCache.Reset()

	inString := false
	var prev byte

	err := jr.eatWhitespace(r)
	if err != nil {
		return err
	}

	ok, _, err := jr.tryLiteral(r)
	if err != nil {
		return err
	}

	// It was a literal, we're done!
	if ok {
		return nil
	}

	// Otherwise it's an array or object
	first := true

	length := 0
	var bs []byte
	for first || jr.eatValueCache.Index() > 0 {
		length++
		first = false

		for {
			bs, err = r.Peek(length)
			if err == bufio.ErrBufferFull {
				_, err = r.Discard(length - 1)
				if err != nil {
					return err
				}

				length = 1
				continue
			}
			if err != nil {
				return err
			}

			break
		}
		b := bs[length-1]
		// b, err := jr.peek(r)
		// if err != nil {
		// 	return err
		// }

		if inString {
			if b == '"' && prev != '\\' {
				inString = false
			}

			// Two \\-es cancel eachother out
			if b == '\\' && prev == '\\' {
				prev = byte(0)
			} else {
				prev = b
			}

			//jr.discard(r)
			continue
		}

		switch b {
		case '[':
			jr.eatValueCache.Append(b)
		case ']':
			c := jr.eatValueCache.Pop()
			if c != '[' {
				return fmt.Errorf("Unexpected end of array: '%s'", string(c))
			}
		case '{':
			jr.eatValueCache.Append(b)
		case '}':
			c := jr.eatValueCache.Pop()
			if c != '{' {
				return fmt.Errorf("Unexpected end of object: '%s'", string(c))
			}
		case '"':
			inString = true
			// Closing quote case handled elsewhere, above
		}

		//jr.discard(r)
		prev = b
	}

	_, err = r.Discard(length)
	return err

	//return nil
}

var (
	trueBytes  = []byte("true")
	falseBytes = []byte("false")
	nullBytes  = []byte("null")
)

func (jr *jsonReader) tryLiteral(r *bufio.Reader) (bool, any, error) {
	c, err := jr.peek(r)
	if err != nil {
		return false, nil, err
	}

	if c == '"' {
		val, err := jr.expectString(r)
		return true, string(val), err
	} else if c == 't' {
		val, err := jr.expectIdentifier(r, trueBytes, true)
		return true, val, err
	} else if c == 'f' {
		val, err := jr.expectIdentifier(r, falseBytes, false)
		return true, val, err
	} else if c == 'n' {
		val, err := jr.expectIdentifier(r, nullBytes, nil)
		return true, val, err
	}

	return jr.tryNumber(r)
}

func (jr *jsonReader) expectValue(r *bufio.Reader, path [][]byte) (any, error) {
	c, err := jr.peek(r)
	if err != nil {
		return nil, err
	}

	if c == '{' {
		return jr.extractDataFromJsonPath(r, path)
	} else if c == '[' {
		return jr.extractArrayDataFromJsonPath(r, path)
	}

	// Can't go any further into a path

	if len(path) != 0 {
		// Reached the end of this object but more of
		// the path remains. So this object doesn't
		// contain this path.
		return nil, nil
	}

	ok, val, err := jr.tryLiteral(r)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("Expected literal, got: '%s'", string(c))
	}

	return val, err
}

func (jr *jsonReader) extractArrayDataFromJsonPath(r *bufio.Reader, path [][]byte) (any, error) {
	n, err := strconv.Atoi(string(path[0]))
	if err != nil {
		return nil, err
	}

	b, err := jr.readByte(r)
	if err != nil {
		return nil, err
	}

	if b != '[' {
		return nil, fmt.Errorf("Expected opening bracket, got: '%s'", string(b))
	}

	var result any
	i := -1
	for {
		i++

		err = jr.eatWhitespace(r)
		if err != nil {
			return nil, err
		}

		b, err := jr.peek(r)
		if err != nil {
			return nil, err
		}

		if b == ']' {
			jr.discard(r)
			break
		}

		if i > 0 {
			if b == ',' {
				jr.discard(r)
			} else {
				return nil, fmt.Errorf("Expected comma between key-value pairs, got: '%s'", string(b))
			}
		}

		err = jr.eatWhitespace(r)
		if err != nil {
			return nil, err
		}

		// If the key is not the start of this path, skip past this value
		if i != n {
			err = jr.eatValue(r)
			if err != nil {
				return nil, err
			}

			continue
		}

		result, err = jr.expectValue(r, path[1:])
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func (jr *jsonReader) extractDataFromJsonPath(r *bufio.Reader, path [][]byte) (any, error) {
	if len(path) == 0 {
		return nil, nil
	}

	err := jr.eatWhitespace(r)
	if err != nil {
		return nil, err
	}

	b, err := jr.readByte(r)
	if err != nil {
		return nil, err
	}

	if b != '{' {
		return nil, fmt.Errorf("Expected opening curly brace, got: '%s'", string(b))
	}

	i := -1

	var result any
	for {
		i++

		err = jr.eatWhitespace(r)
		if err != nil {
			return nil, err
		}

		b, err := jr.peek(r)
		if err != nil {
			return nil, err
		}

		if b == '}' {
			jr.discard(r)
			break
		}

		if i > 0 {
			if b == ',' {
				jr.discard(r)
			} else {
				return nil, fmt.Errorf("Expected comma between key-value pairs, got: '%s'", string(b))
			}
		}

		err = jr.eatWhitespace(r)
		if err != nil {
			return nil, err
		}

		// Grab the key
		s, err := jr.expectString(r)
		if err != nil {
			return nil, err
		}

		err = jr.eatWhitespace(r)
		if err != nil {
			return nil, err
		}

		b, err = jr.readByte(r)
		if err != nil {
			return nil, err
		}

		if b != ':' {
			return nil, fmt.Errorf("Expected colon, got: '%s'", string(b))
		}

		err = jr.eatWhitespace(r)
		if err != nil {
			return nil, err
		}

		// If the key is not the start of this path, skip past this value
		if !bytes.Equal(path[0], s) {
			err = jr.eatValue(r)
			if err != nil {
				return nil, err
			}

			continue
		}

		result, err = jr.expectValue(r, path[1:])
		if err != nil {
			return nil, err
		}
	}

	return result, nil
}

func main() {
	//defer profile.Start().Stop()
	//defer profile.Start(profile.MemProfile).Stop()

	var nonFlagArgs []string

	for _, arg := range os.Args[1:] {
		if arg == "--debug" || arg == "-d" {
			DEBUG = true
			continue
		}

		nonFlagArgs = append(nonFlagArgs, arg)
	}

	if DEBUG {
		debugln = _debugln
		debug = _debug
	}

	pathS := strings.Split(nonFlagArgs[0], ".")
	if pathS[0] == "" {
		pathS = pathS[1:]
	}
	var path [][]byte
	for _, part := range pathS {
		path = append(path, []byte(part))
	}

	b := bufio.NewReader(os.Stdin)
	enc := json.NewEncoder(os.Stdout)

	var jr jsonReader
	var val any
	var err error

	for {
		jr.reset()

		val, err = jr.extractDataFromJsonPath(b, path)
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Println("Read", string(jr.read.List()))
			log.Fatalln(err)
		}

		err = enc.Encode(val)
		if err != nil {
			log.Fatalln(err)
		}
	}
}
