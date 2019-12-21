package main

import (
	"fmt"
	"io/ioutil"
	"sort"
	"strconv"
	"strings"
	"unsafe"

	"github.com/gonutz/tic"
)

func main() {
	smallTest()
	data, err := ioutil.ReadFile("german.txt")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	sort.Strings(lines)
	fmt.Println(len(lines), "words")
	n := len(lines)
	lines = lines[:n]
	var dawg *node
	func() {
		defer tic.Toc()("build DAWG")
		b := newDawgBuilder()
		for _, line := range lines {
			b.add(line)
		}
		b.finish()
		fmt.Println(len(b.register), "states registered")
		dawg = b.root
	}()
	defer tic.Toc()("check words")
	for _, w := range lines {
		if !containsWord(dawg, w) {
			panic("word not contained: " + w)
		}
		if containsWord(dawg, "x"+w+"x") {
			panic("word contained: x" + w + "x")
		}
	}
}

func smallTest() {
	b := newDawgBuilder()
	b.add("tap")
	b.add("taps")
	b.add("top")
	b.add("tops")
	b.finish()
	printDawg(b.root)
}

func printDawg(n *node) {
	printIndented(n, "")
}

func printIndented(n *node, indent string) {
	fmt.Printf("%s0x%x\n", indent, uintptr(unsafe.Pointer(n)))
	for _, e := range n.edges {
		final := ""
		if e.dest.final {
			final = "!"
		}
		fmt.Printf("%s%s%s\n", indent, string(e.letter), final)
		printIndented(e.dest, indent+"   ")
	}
}

func newDawgBuilder() *dawgBuilder {
	return &dawgBuilder{
		root:     &node{},
		register: make(map[string]*node),
	}
}

type dawgBuilder struct {
	root     *node
	lastWord string
	register map[string]*node
}

func (b *dawgBuilder) add(word string) {
	if word == b.lastWord {
		return // word is already in there
	}
	if !(word > b.lastWord) {
		panic("words must be added in lexicographical order")
	}
	prefix := commonPrefix(word, b.lastWord)
	suffix := []rune(strings.TrimPrefix(word, string(prefix)))
	lastState := b.root
	for range prefix {
		lastState = lastState.edges[len(lastState.edges)-1].dest
	}
	if len(lastState.edges) > 0 {
		b.replaceOrRegister(lastState)
	}
	b.addSuffix(lastState, suffix)
	b.lastWord = word
}

func commonPrefix(s1, s2 string) []rune {
	// make sure s1 is shorter or of equal length
	if len(s1) > len(s2) {
		return commonPrefix(s2, s1)
	}
	a, b := []rune(s1), []rune(s2)
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[:i] // mismatch, return everything before this rune
		}
	}
	return a // return the shorter one
}

func (b *dawgBuilder) replaceOrRegister(n *node) {
	child := n.edges[len(n.edges)-1].dest
	if !child.registered {
		if len(child.edges) > 0 {
			b.replaceOrRegister(child)
		}

		if q := b.findRegistered(child); q != nil {
			n.edges[len(n.edges)-1].dest = q
		} else {
			b.addToRegister(child)
			child.registered = true
		}
	}
}

func (b *dawgBuilder) addToRegister(n *node) {
	hash := buildNodeHash(n)
	b.register[hash] = n
}

func buildNodeHash(n *node) string {
	var hash string
	if n.final {
		hash += "!"
	}
	hash += strconv.Itoa(len(n.edges))
	for _, e := range n.edges {
		hash += string(e.letter) + "|" + strconv.Itoa(int(uintptr(unsafe.Pointer(e.dest))))
	}
	return hash
}

func (b *dawgBuilder) findRegistered(n *node) *node {
	hash := buildNodeHash(n)
	return b.register[hash]
}

func (b *dawgBuilder) addSuffix(n *node, word []rune) {
	for _, r := range word {
		m := &node{}
		n.edges = append(n.edges, &edge{letter: r, dest: m})
		n = m
	}
	n.final = true
}

func (b *dawgBuilder) finish() {
	b.replaceOrRegister(b.root)
}

type node struct {
	final      bool
	registered bool
	edges      []*edge
}

type edge struct {
	letter rune
	dest   *node
}

func containsWord(n *node, word string) bool {
	findEdge := func(n *node, r rune) *edge {
		// We use a binary search over the edges' letters. Since the input into
		// building the DAWG is sorted lexicographically, so are the edge
		// letters.
		// Tests show that this saves about 7% time compared to linear search of
		// the edge list.
		left, right := 0, len(n.edges)-1
		for left <= right {
			m := (left + right) / 2
			if n.edges[m].letter < r {
				left = m + 1
			} else if n.edges[m].letter > r {
				right = m - 1
			} else {
				return n.edges[m]
			}
		}
		return nil
	}

	for _, r := range word {
		e := findEdge(n, r)
		if e == nil {
			return false
		}
		n = e.dest
	}
	return n.final
}
