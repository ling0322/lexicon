package reimu

import (
	"math/rand"
	"testing"
)


const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

// sampleT stores the samples for testing
type sampleT struct {
	key string
	value int32
}

// randomString generates a random string shorter than maxLength
func randomString(maxLength int) string {
	length := rand.Intn(maxLength) + 1
	bytes := make([]byte, length)
	for i := 0; i < length; i++ {
		bytes[i] = letters[rand.Intn(len(letters))]
	}
	return string(bytes)
}

// prepareData prepares the testing data
func prepareData(N, maxLen int) (dict map[string]int32, testData []sampleT) {
	// Prepare dict
	dict = map[string]int32{}
	for i := 0; i < N / 2; i++ {
		rnds := randomString(maxLen)
		if _, ok := dict[rnds]; !ok {
			dict[rnds] = int32(i)
			testData = append(testData, sampleT{rnds, int32(i)})
		}
	}

	// Negative samples
	for len(testData) < N {
		rnds := randomString(maxLen)
		if _, ok := dict[rnds]; !ok {
			testData = append(testData, sampleT{rnds, -1})
		}
	}

	return
}

func BenchmarkLexicon(b *testing.B) {
	kMaxLen := 25

	b.StopTimer()
	// Prepare testing data
	dict, testData := prepareData(b.N, kMaxLen)

	// Build lexicon
	lexicon, err := Build(dict, nil)
	if err != nil {
		b.FailNow()
	}
	b.StartTimer()

	sum := int32(0)
	for _, sample := range testData {
		v, _ := lexicon.Get(sample.key)
		sum += v
	}
}

func BenchmarkGoMap(b *testing.B) {
	kMaxLen := 25

	b.StopTimer()
	// Prepare testing data
	dict, testData := prepareData(b.N, kMaxLen)
	b.StartTimer()

	sum := int32(0)
	for _, sample := range testData {
		sum += dict[sample.key]
	}
}

func TestRandomString(t *testing.T) {
	const N = 10000
	const kMaxLen = 25

	// Prepare testing data
	dict, testData := prepareData(N, kMaxLen)

	// Builds lexicon
	lexicon, err := Build(dict, nil)
	if err != nil {
		t.FailNow()
	}

	err = lexicon.Save("lexicon.reimu")
	if err != nil {
		t.FailNow()
	}
	lexicon, err = Read("lexicon.reimu")
	if err != nil {
		t.FailNow()
	}

	for _, sample := range testData {
		v, ok := lexicon.Get(sample.key)
		if sample.value == -1 && ok {
			t.FailNow()
		}
		if sample.value >= 0 && (!ok || v != sample.value) {
			t.FailNow()
		} 
	}
}
