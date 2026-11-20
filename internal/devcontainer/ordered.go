package devcontainer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// node is a JSON value that keeps object members in document order. Feature
// install order and generated ENV order both depend on that order, which
// map-based decoding discards.
type node struct {
	kind    nodeKind
	str     string
	num     json.Number
	boolean bool
	members []member
	items   []node
}

type member struct {
	key   string
	value node
}

type nodeKind int

const (
	kindNull nodeKind = iota
	kindString
	kindNumber
	kindBool
	kindObject
	kindArray
)

func (k nodeKind) String() string {
	return [...]string{"null", "string", "number", "boolean", "object", "array"}[k]
}

func decodeOrdered(data []byte) (node, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	n, err := readNode(dec)
	if err != nil {
		return node{}, err
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return node{}, errors.New("trailing data after the top-level value")
	}
	return n, nil
}

func readNode(dec *json.Decoder) (node, error) {
	tok, err := dec.Token()
	if err != nil {
		return node{}, err
	}
	switch t := tok.(type) {
	case nil:
		return node{kind: kindNull}, nil
	case string:
		return node{kind: kindString, str: t}, nil
	case json.Number:
		return node{kind: kindNumber, num: t}, nil
	case bool:
		return node{kind: kindBool, boolean: t}, nil
	case json.Delim:
		switch t {
		case '{':
			return readObject(dec)
		case '[':
			return readArray(dec)
		}
	}
	return node{}, fmt.Errorf("unexpected token %v", tok)
}

func readObject(dec *json.Decoder) (node, error) {
	obj := node{kind: kindObject}
	for dec.More() {
		keyTok, err := dec.Token()
		if err != nil {
			return node{}, err
		}
		key, ok := keyTok.(string)
		if !ok {
			return node{}, fmt.Errorf("object key %v is not a string", keyTok)
		}
		value, err := readNode(dec)
		if err != nil {
			return node{}, err
		}
		obj.members = append(obj.members, member{key: key, value: value})
	}
	if _, err := dec.Token(); err != nil {
		return node{}, err
	}
	return obj, nil
}

func readArray(dec *json.Decoder) (node, error) {
	arr := node{kind: kindArray}
	for dec.More() {
		item, err := readNode(dec)
		if err != nil {
			return node{}, err
		}
		arr.items = append(arr.items, item)
	}
	if _, err := dec.Token(); err != nil {
		return node{}, err
	}
	return arr, nil
}

// scalarString renders a scalar the way a shell environment would carry it:
// devcontainer feature options are delivered to install.sh as env vars, so
// `true` becomes "true" and 20 becomes "20".
func (n node) scalarString() (string, bool) {
	switch n.kind {
	case kindString:
		return n.str, true
	case kindNumber:
		return n.num.String(), true
	case kindBool:
		if n.boolean {
			return "true", true
		}
		return "false", true
	default:
		return "", false
	}
}
