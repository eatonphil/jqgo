package main

import (
	"io"
	"strconv"
	"log"
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

var DEBUG = false

func debug(msg string, args... any) {
	if !DEBUG {
		return
	}

	log.Printf(msg, args...)
}

func debugln(args... any) {
	if !DEBUG {
		return
	}

	log.Println(args...)
}

func eatWhitespace(r *bufio.Reader) error {
	for {
		bs, err := r.Peek(1)
		if err != nil {
			return err
		}

		isWhitespace := bs[0] == ' ' ||
			bs[0] == '\n' ||
			bs[0] == '\t' ||
			bs[0] == '\r'
		if !isWhitespace {
			return nil
		}

		_, err = r.ReadByte()
		if err != nil {
			return err
		}
	}
}

func expectString(r *bufio.Reader) (string, error) {
	err := eatWhitespace(r)
	if err != nil {
		return "", err
	}

	b, err := r.ReadByte()
	if err != nil {
		return "", err
	}

	if b != '"' {
		return "", fmt.Errorf("Expected double quote to start string, got: '%s'", string(b))
	}

	var s []byte
	var prev byte
	for {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}

		if b == '"' {
			// Overwrite the escaped double quote
			if prev == '\\' {
				s[len(s)-1] = '"'
			} else {
				// Otherwise it's the actual end
				break
			}
		}

		s = append(s, b)
		prev = b
	}

	return string(s), nil
}

func expectIdentifier(r *bufio.Reader, ident string, value any) (any, error) {
	bs, err := r.Peek(len(ident))
	if err != nil {
		return nil, err
	}

	if string(bs) == ident {
		// Read everything we peeked at
		for i := 0; i < len(ident); i++ {
			_, err = r.ReadByte()
			if err != nil {
				return nil, err
			}
		}
		return value, nil
	}

	return nil, fmt.Errorf("Unknown value: '%s'", string(bs))
}

func tryNumber(r *bufio.Reader) (bool, any, error) {
	var numberBytes []byte
	var hint []byte

	for {
		bs, err := r.Peek(1)
		if err != nil {
			return false, nil, err
		}

		c := bs[0]
		hint = append(hint, c)

		isNumberCharacter := (c >= '0' && c <= '9') || c == 'e' || c == '-'
		if !isNumberCharacter {
			break
		}

		numberBytes = append(numberBytes, c)

		_, err = r.ReadByte()
		if err != nil {
			return false, nil, err
		}
	}

	if len(numberBytes) == 0 {
		return false, nil, nil
	}

	var n float64
	err := json.Unmarshal(numberBytes, &n)
	return true, n, err
}

func eatValue(r *bufio.Reader) error {
	var stack []string
	inString := false
	var prev byte

	err := eatWhitespace(r)
	if err != nil {
		return err
	}

	ok, _, err := tryLiteral(r)
	if err != nil {
		return err
	}

	// It was a literal, we're done!
	if ok {
		return nil
	}

	// Otherwise it's an array or object
	first := true

	for first || len(stack) > 0 {
		first = false

		bs, err := r.Peek(1)
		if err != nil {
			return err
		}
		b := bs[0]

		if inString {
			if b == '"' {
				if prev == '\\' {
					prev = b
					continue
				}

				inString = false
			}
		}

		switch b {
		case '[':
			stack = append(stack, "[")
		case ']':
			if stack[len(stack)-1] != "[" {
				return fmt.Errorf("Unexpected end of array")
			}

			stack = stack[:len(stack)-1]
		case '{':
			stack = append(stack, "{")
		case '}':
			if stack[len(stack)-1] != "{" {
				return fmt.Errorf("Unexpected end of object")
			}

			stack = stack[:len(stack)-1]
		case '"':
			inString = true
			// Closing quote case handled elsewhere, above
		}

		_, err = r.ReadByte()
		if err != nil {
			return err
		}

		prev = b
	}

	return nil
}

func tryLiteral(r *bufio.Reader) (bool, any, error) {
	bs, err := r.Peek(1)
	if err != nil {
		return false, nil, err
	}

	c := bs[0]

	if c == '"' {
		val, err := expectString(r)
		return true, val, err
	} else if c == 't' {
		val, err := expectIdentifier(r, "true", true)
		return true, val, err
	} else if c == 'f' {
		val, err := expectIdentifier(r, "false", false)
		return true, val, err
	} else if c == 'n' {
		val, err := expectIdentifier(r, "null", nil)
		return true, val, err
	}

	return tryNumber(r)
}

func expectValue(r *bufio.Reader, path []string) (any, error) {
	bs, err := r.Peek(1)
	if err != nil {
		return nil, err
	}
	c := bs[0]

	if c == '{' {
		return extractDataFromJsonPath(r, path)
	} else if c == '[' {
		return extractArrayDataFromJsonPath(r, path)
	}

	// Can't go any further into a path

	if len(path) != 0 {
		// Reached the end of this object but more of
		// the path remains. So this object doesn't
		// contain this path.
		return nil, nil
	}

	ok, val, err := tryLiteral(r)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("Expected literal, got: '%s'", string(c))
	}

	return val, err
}

func extractArrayDataFromJsonPath(r *bufio.Reader, path []string) (any, error) {
	n, err := strconv.Atoi(path[0])
	if err != nil {
		return nil, err
	}

	b, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	if b != '[' {
		return nil, fmt.Errorf("Expected opening bracket, got: '%s'", string(b))
	}

	i := -1
	for {
		i++

		err = eatWhitespace(r)
		if err != nil {
			return nil, err
		}

		bs, err := r.Peek(1)
		if err != nil {
			return nil, err
		}

		if bs[0] == '}' {
			_, err := r.ReadByte()
			if err != nil {
				return nil, err
			}
			break
		}

		if i > 0 {
			if bs[0] == ',' {
				_, err := r.ReadByte()
				if err != nil {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("Expected comma between key-value pairs, got: '%s'", string(bs))
			}
		}

		err = eatWhitespace(r)
		if err != nil {
			return nil, err
		}

		// If the key is not the start of this path, skip past this value
		if i != n {
			debugln("Skipping index", i)
			err = eatValue(r)
			if err != nil {
				return nil, err
			}

			debugln("Ate value at index", i)
			continue
		}

		return expectValue(r, path[1:])
	}

	return nil, nil
}

func extractDataFromJsonPath(r *bufio.Reader, path []string) (any, error) {
	if len(path) == 0 {
		return nil, nil
	}

	err := eatWhitespace(r)
	if err != nil {
		return nil, err
	}

	b, err := r.ReadByte()
	if err != nil {
		return nil, err
	}

	if b != '{' {
		return nil, fmt.Errorf("Expected opening curly brace, got: '%s'", string(b))
	}

	i := -1
	for {
		i++

		err = eatWhitespace(r)
		if err != nil {
			return nil, err
		}

		bs, err := r.Peek(1)
		if err != nil {
			return nil, err
		}

		if bs[0] == '}' {
			_, err := r.ReadByte()
			if err != nil {
				return nil, err
			}
			break
		}

		if i > 0 {
			if bs[0] == ',' {
				_, err := r.ReadByte()
				if err != nil {
					return nil, err
				}
			} else {
				return nil, fmt.Errorf("Expected comma between key-value pairs, got: '%s'", string(bs))
			}
		}

		err = eatWhitespace(r)
		if err != nil {
			return nil, err
		}

		// Grab the key
		debugln("Grabbing string")
		s, err := expectString(r)
		if err != nil {
			return nil, err
		}

		debugln("Grabbed string", s)

		err = eatWhitespace(r)
		if err != nil {
			return nil, err
		}

		b, err = r.ReadByte()
		if err != nil {
			return nil, err
		}

		if b != ':' {
			return nil, fmt.Errorf("Expected colon, got: '%s'", string(b))
		}

		err = eatWhitespace(r)
		if err != nil {
			return nil, err
		}

		// If the key is not the start of this path, skip past this value
		if path[0] != s {
			debugln("Skipping key", s)
			err = eatValue(r)
			if err != nil {
				return nil, err
			}

			debugln("Ate value", s)
			continue
		}

		return expectValue(r, path[1:])
	}

	// Path not found
	return nil, nil
}

func main() {
	var nonFlagArgs []string

	for _, arg := range os.Args[1:] {
		if arg == "--debug" || arg == "-d" {
			DEBUG = true
			continue
		}

		nonFlagArgs = append(nonFlagArgs, arg)
	}
	
	b := bufio.NewReader(os.Stdin)
	val, err := extractDataFromJsonPath(b, strings.Split(nonFlagArgs[0], "."))
	if err != nil && err != io.EOF {
		log.Fatalln(err)
	}

	enc := json.NewEncoder(os.Stdout)
	err = enc.Encode(val)
	if err != nil {
		log.Fatalln(err)
	}
}
