package reimu

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"os"
)

const Header = "REIMU_Lex.v1"
const ProgressStep = 4096

// Lexicon is the double array implementation of a trie-based lexicon
type Lexicon struct {
	slots []slotT

	suffixIndex []int32
	suffixValue []int32
	suffix      []byte

	// Free blocks are the blocks which have free slots. Only be used in trie
	// building. Here freeBlocks should be an array to keep blocks in order
	freeBlocks []*blockT

	// Only used to display progress
	totalNodes     int
	processedNodes int
}

// State keeps the state in traversing the trie
type State struct {
	state     int32
	suffixId  int32
	suffixPtr int32
}

// slotT in one cell in double array trie, constsis of two values: base and
// check
type slotT struct {
	Base  int32
	Check int32
}

// A block represents the information of 256 slots in reimu-trie
type blockT struct {
	blockId   int
	freeSlots int
}

// InitialState creates the initial state in traversing
func InitialState() State {
	return State{
		state:     0,
		suffixId:  -1,
		suffixPtr: -1,
	}
}

// empty returns whether this slot is free
func (s *slotT) empty() bool {
	return s.Check < 0
}

// newLexicon creates a new instance of Lexicon
func newLexicon() *Lexicon {
	t := &Lexicon{
		slots:       []slotT{},
		suffixIndex: []int32{},
		suffixValue: []int32{},
		suffix:      []byte{},
		freeBlocks:  []*blockT{},
	}
	return t
}

// addBlock adds a new block into reimu-trie, returns the index of created
// block
func (t *Lexicon) addBlock() int {
	block := make([]slotT, 256)
	for i := range block {
		block[i].Check = -1
	}

	numBlocks := len(t.slots) / 256

	t.slots = append(t.slots, block...)
	t.freeBlocks = append(t.freeBlocks, &blockT{
		blockId:   numBlocks,
		freeSlots: 256,
	})

	return numBlocks
}

// findSuitableBase finds a base in slots to put child-nodes of given node
func (t *Lexicon) findSuitableBase(node *_Trie) int {
	assert(!node.isEmpty() && !node.hasSuffix, "findSuitableBase: invalid node")
	children := make([]byte, 0, 256)
	for child := range node.children {
		children = append(children, child)
	}
	// Value node is in child 0
	if node.hasValue {
		children = append(children, 0)
	}

	for _, b := range t.freeBlocks {
		if b.freeSlots >= len(children) {
			// This node has adequate free slots for child-nodes. Then check
			// whether all nodes could be placed well
			for base := b.blockId * 256; base < (b.blockId+1)*256; base++ {
				success := true
				for _, child := range children {
					s := base ^ int(child)
					if !t.slots[s].empty() {
						// If slots[s] already have value
						success = false
						break
					}
				}
				if success {
					return base
				}
			}
		}
	}

	// Seems no block is suitable for this node, needs to create a new block.
	// Since the new added block is an empty block, we can use the first slot
	// in this block directly
	blockId := t.addBlock()
	return blockId * 256
}

// build builds the reimu-trie from trie, returns the base value of this node in
// double array trie
func (t *Lexicon) build(
	node *_Trie,
	fromState int32,
	progress func(int, int)) int32 {
	// Add a new block when didn't have free blocks
	if len(t.freeBlocks) == 0 {
		t.addBlock()
	}

	// Display progress when needed
	t.processedNodes++
	if progress != nil && t.processedNodes % ProgressStep == 0 {
		progress(t.processedNodes, t.totalNodes)
	}

	if node.hasSuffix {
		// If this node is a suffix node
		suffixId := len(t.suffixValue)
		t.suffixValue = append(t.suffixValue, node.value)
		t.suffixIndex = append(t.suffixIndex, int32(len(t.suffix)))

		suffixBytes := make([]byte, len(node.suffix)+1)
		copy(suffixBytes, node.suffix)
		suffixBytes[len(suffixBytes)-1] = '\x00'
		t.suffix = append(t.suffix, suffixBytes...)

		// Negative value in base indicates its a index in suffixValue
		// If index in suffixValue & suffixValue is i, then base = -i - 1
		return int32(-suffixId - 1)
	} else {
		base := t.findSuitableBase(node)
		slotsRequired := 0

		// Value node
		if node.hasValue {
			assert(t.slots[base].empty(), "buildLexicon: invalid base value")
			t.slots[base].Base = node.value
			t.slots[base].Check = fromState
			slotsRequired++
		}

		// Set 'check' array for children. This step also mark child-slots
		// as 'used'
		for b := range node.children {
			s := base ^ int(b)
			assert(t.slots[s].empty(), "buildLexicon: invalid base value")
			t.slots[s].Check = fromState

			slotsRequired++
		}

		// Update block state
		blockId := base / 256
		blockUpdated := false
		for i, block := range t.freeBlocks {
			if block.blockId == blockId {
				blockUpdated = true
				block.freeSlots -= slotsRequired
				assert(block.freeSlots >= 0, "buildLexicon: invalid block.freeSlots")

				if block.freeSlots == 0 {
					// Ok, we need to remove this block from freeBlocks
					t.freeBlocks = append(t.freeBlocks[:i], t.freeBlocks[i+1:]...)
				}
				break
			}
		}
		assert(blockUpdated, "buildLexicon: block not exist")

		// Set 'base' array for children. Also recursively calling
		// buildLexicon() for child-nodes
		for b, childNode := range node.children {
			s := base ^ int(b)
			t.slots[s].Base = t.build(childNode, int32(s), progress)
		}

		return int32(base)
	}
}

// Build builds the reimu-trie from dict
func Build(dict map[string]int32, progress func(int, int)) (*Lexicon, error) {
	trie, err := buildTrie(dict)
	if err != nil {
		return nil, err
	}

	Lexicon := newLexicon()
	Lexicon.totalNodes = trie.countNode()

	// Prepare the root node in Lexicon
	Lexicon.addBlock()
	Lexicon.slots[0] = slotT{
		Base:  0,
		Check: 0,
	}
	Lexicon.freeBlocks[0].freeSlots = 255
	if len(dict) == 0 {
		// If it is an empty dict, just return an empty lexicon
		return Lexicon, nil
	}

	rootBase := Lexicon.build(trie, 0, progress)
	assert(rootBase == 0, "Build: invalid rootBase")

	if progress != nil {
		progress(Lexicon.totalNodes, Lexicon.totalNodes)
	}

	return Lexicon, nil
}

// Traverse traverses the Lexicon by character list 'key' from state 's'.
// Returns values by different conditions are:
//   - Traverse success & final state have value:
//       value = <value>, ok = true, s.Valid() = true
//   - Traverse success & final state no value:
//       value = UNDEFINED, ok = false, s.Valid() = true
//   - Traverse failed
//       value = UNDEFINED, ok = false, s.Valid() = false
func (t *Lexicon) Traverse(key string, s *State) (value int32, ok bool) {
	for i := 0; i < len(key); i++ {
		// NULL char is not allowed in Reimu-trie
		b := key[i]
		if b == '\x00' {
			return -1, false
		}

		if s.state >= 0 {
			// In double array
			base := t.slots[s.state].Base
			if base >= 0 {
				nextState := base ^ int32(b)
				// Still in double array
				if t.slots[nextState].Check != s.state {
					s.state = -1
					s.suffixId = -1
					return -1, false
				}
				s.state = nextState
				continue
			} else {
				// Switch to suffix
				s.state = -1
				s.suffixId = -base - 1
				s.suffixPtr = t.suffixIndex[s.suffixId]
			}
		}

		if s.suffixId >= 0 {
			// In suffix
			if b != t.suffix[s.suffixPtr] {
				s.state = -1
				s.suffixId = -1
				return -1, false
			}
			s.suffixPtr++
		}
	}

	// Traverse finished, get values
	if s.state >= 0 {
		base := t.slots[s.state].Base
		if base < 0 || t.slots[base].Check != s.state {
			return -1, false
		} else {
			return t.slots[base].Base, true
		}
	} else if s.suffixId >= 0 {
		if t.suffix[s.suffixPtr] == '\x00' {
			return t.suffixValue[s.suffixId], true
		} else {
			return -1, false
		}
	}

	return -1, false
}

// Get gets the value by key in Lexicon. On success, returns (value, true).
// On failed, returns (ok = false)
func (t *Lexicon) Get(key string) (value int32, ok bool) {
	s := InitialState()
	return t.Traverse(key, &s)
}

// Read reads reimu-trie from file
func Read(filename string) (*Lexicon, error) {
	t := new(Lexicon)
	fd, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer fd.Close()

	// Function to call binary.Read
	binaryRead := func(dataPtr interface{}, previousErr error) error {
		if previousErr != nil {
			return previousErr
		}

		err := binary.Read(fd, binary.LittleEndian, dataPtr)
		return err
	}

	header := make([]byte, len(Header))
	err = binaryRead(&header, err)
	if err == nil && string(header) != Header {
		return nil, errors.New(fmt.Sprintf("Corrupted file: %s", filename))
	}

	var numSlots int32
	var numSuffix int32
	var numSuffixBytes int32
	err = binaryRead(&numSlots, err)
	err = binaryRead(&numSuffix, err)
	err = binaryRead(&numSuffixBytes, err)
	if err != nil {
		return nil, err
	}

	t.slots = make([]slotT, numSlots)
	t.suffixIndex = make([]int32, numSuffix)
	t.suffixValue = make([]int32, numSuffix)
	t.suffix = make([]byte, numSuffixBytes)
	err = binaryRead(&t.slots, err)
	err = binaryRead(&t.suffixIndex, err)
	err = binaryRead(&t.suffixValue, err)
	err = binaryRead(&t.suffix, err)

	return t, err
}

// Save saves the reimu-trie to file
func (t *Lexicon) Save(filename string) error {
	fd, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer fd.Close()

	// function to call binary.Write
	binaryWrite := func(data interface{}, previousErr error) error {
		if previousErr != nil {
			return previousErr
		}

		err := binary.Write(fd, binary.LittleEndian, data)
		return err
	}

	err = binaryWrite([]byte(Header), err)
	err = binaryWrite(int32(len(t.slots)), err)
	err = binaryWrite(int32(len(t.suffixIndex)), err)
	err = binaryWrite(int32(len(t.suffix)), err)
	err = binaryWrite(t.slots, err)
	err = binaryWrite(t.suffixIndex, err)
	err = binaryWrite(t.suffixValue, err)
	err = binaryWrite(t.suffix, err)

	return err
}

// ProgressBar prints a progress bar with processed and total
func ProgressBar(processed, total int) {
	const barWidth = 64

	if processed >= total {
		fmt.Printf("\r[%s] Done     \n", strings.Repeat("=", barWidth))
	} else {
		fmt.Printf("\r[")
		pos := barWidth * processed / total
		fmt.Printf("%s", strings.Repeat("=", pos))
		switch processed / ProgressStep % 4 {
		case 0:
			fmt.Printf("-")
		case 1:
			fmt.Printf("\\")
		case 2:
			fmt.Printf("|")
		case 3:
			fmt.Printf("/")
		}
		precentage := float64(processed) / float64(total) * 100.0
		fmt.Printf(
			"%s] % 3.2f%%\r",
			strings.Repeat(" ", barWidth - pos - 1),
			precentage)
	}
}

