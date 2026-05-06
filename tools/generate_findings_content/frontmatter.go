package main

import (
	"bytes"
	"errors"

	"gopkg.in/yaml.v3"
)

func parseFrontMatter(data []byte) (frontMatter, []byte, error) {
	data = bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
	if !bytes.HasPrefix(data, []byte("---\n")) {
		return frontMatter{}, nil, errors.New("content markdown must start with YAML front matter")
	}
	head, body, ok := bytes.Cut(data[len("---\n"):], []byte("\n---\n"))
	if !ok {
		return frontMatter{}, nil, errors.New("front matter is missing closing ---")
	}

	var front frontMatter
	dec := yaml.NewDecoder(bytes.NewReader(head))
	dec.KnownFields(true)
	if err := dec.Decode(&front); err != nil {
		return frontMatter{}, nil, err
	}
	return front, body, nil
}
