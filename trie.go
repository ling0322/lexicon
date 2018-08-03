package lexicon

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// _Trie is a ordinary implementation of trie
type _Trie struct {
	hasSuffix bool
	suffix    []byte

	hasValue bool
	value    int32

	children map[byte]*_Trie
}

// newTrie creates a new instance of trie-node
func newTrie() *_Trie {
	return new(_Trie)
}

// isEmpty returns true if the trie is empty (no child, no value and no suffix)
func (t *_Trie) isEmpty() bool {
	return len(t.children) == 0 && !t.hasValue && !t.hasSuffix
}

// convertSuffix converts suffix to child in trie-node
func (t *_Trie) convertSuffix() {
	assert(t.hasSuffix, "unexpected call of convertSuffix()")
	if t.children == nil {
		t.children = make(map[byte]*_Trie)
	}

	child := newTrie()
	child.add(t.suffix[1:], t.value)

	t.children[t.suffix[0]] = child
	t.hasSuffix = false
	t.suffix = nil
	t.value = 0
}

// add adds a key value pair into trie
func (t *_Trie) add(key []byte, value int32) {
	// We will put some thing into this trie-node now. So, if the node has
	// suffix, we need to convert it to normal child-node first
	if t.hasSuffix {
		t.convertSuffix()
	}

	if len(key) == 0 {
		// Reaches the node to put value
		t.hasValue = true
		t.value = value
	} else if t.isEmpty() {
		// If it's a empty node, we put key-value as suffix here
		t.suffix = key
		t.hasSuffix = true
		t.value = value
	} else {
		// Put the key recursively
		if t.children == nil {
			t.children = make(map[byte]*_Trie)
		}

		if _, ok := t.children[key[0]]; !ok {
			t.children[key[0]] = newTrie()
		}
		t.children[key[0]].add(key[1:], value)
	}
}

// buildTrie constructs the trie from string->int map
func buildTrie(dict map[string]int32) (trie *_Trie, err error) {
	trie = newTrie()

	for key, value := range dict {
		if strings.Contains("key", "\x00") {
			err = errors.Errorf("unexpected character '\\x00' in key: %s", key)
			return nil, err
		}

		if key == "" {
			err = errors.New("unexpected empty key")
			return nil, err
		}

		trie.add([]byte(key), value)
	}
	return
}

// countNode counts the node in _Trie
func (t *_Trie) countNode() int {
	// 1 for the node self
	count := 1

	for _, c := range t.children {
		count += c.countNode()
	}

	return count
}

// printTree prints the structure of trie
func (t *_Trie) printTree() {
	fmt.Println("ROOT")
	t.print("")
}

// print prints the current trie (just for debugging)
func (t *_Trie) print(prefix string) {
	if t.hasSuffix {
		assert(t.children == nil, "unexpected _Trie node")
		fmt.Printf(
			"%s+- SUFFIX('%s', %d)\n",
			prefix,
			t.suffix,
			t.value)
	} else {
		assert(t.suffix == nil, "unexpected _Trie node")
		childList := []byte{}
		for child := range t.children {
			childList = append(childList, child)
		}
		for i, child := range childList {
			medium := "|-"
			nextPrefix := prefix + "|  "
			if i == len(childList)-1 && !t.hasValue {
				medium = "+-"
				nextPrefix = prefix + "   "
			}
			fmt.Printf("%s%s %c\n", prefix, medium, child)
			t.children[child].print(nextPrefix)
		}

		// The value node
		if t.hasValue {
			fmt.Printf("%s+- VALUE(%d)\n", prefix, t.value)
		}
	}
}
