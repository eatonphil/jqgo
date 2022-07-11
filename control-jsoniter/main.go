package main

import (
	"io"
	"log"
	"os"
	"strconv"
	"strings"

	json "github.com/json-iterator/go"
)

func extractValueAtPath(a map[string]any, path []string) (any, error) {
	if len(path) == 0 {
		return nil, nil
	}

	var v any = a

	for _, part := range path {
		if arr, ok := v.([]any); ok {
			n, err := strconv.Atoi(part)
			if err != nil {
				return nil, err
			}

			v = arr[n]
			continue
		}

		m, ok := v.(map[string]any)
		if !ok {
			// Path into a non-map
			return nil, nil
		}

		v, ok = m[part]
		if !ok {
			// Path does not exist
			return nil, nil
		}
	}

	return v, nil
}

func main() {
	path := strings.Split(os.Args[1], ".")
	if path[0] == "" {
		path = path[1:]
	}

	dec := json.NewDecoder(os.Stdin)
	var a map[string]any

	enc := json.NewEncoder(os.Stdout)

	for {
		err := dec.Decode(&a)
		if err == io.EOF {
			break
		}

		if err != nil {
			log.Fatal(err)
		}

		v, err := extractValueAtPath(a, path)
		if err != nil {
			log.Fatal(err)
		}

		err = enc.Encode(v)
		if err != nil {
			log.Fatal(err)
		}
	}
}
