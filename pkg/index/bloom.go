package index

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/cespare/xxhash/v2"
)

// BloomFilter is a space-efficient probabilistic data structure for set membership testing.
type BloomFilter struct {
	bits    []uint64
	numHash int
	size    uint64 // number of bits
}

// NewBloomFilter creates a bloom filter sized for expectedItems with the given false positive rate.
func NewBloomFilter(expectedItems int, fpRate float64) *BloomFilter {
	if expectedItems <= 0 {
		expectedItems = 1
	}
	if fpRate <= 0 || fpRate >= 1 {
		fpRate = 0.01
	}

	// Optimal number of bits: m = -n * ln(p) / (ln2)^2
	n := float64(expectedItems)
	m := -n * math.Log(fpRate) / (math.Ln2 * math.Ln2)
	size := uint64(math.Ceil(m))
	if size == 0 {
		size = 64
	}

	// Optimal number of hash functions: k = (m/n) * ln2
	k := int(math.Ceil((float64(size) / n) * math.Ln2))
	if k < 1 {
		k = 1
	}

	numWords := (size + 63) / 64
	return &BloomFilter{
		bits:    make([]uint64, numWords),
		numHash: k,
		size:    size,
	}
}

// Add inserts a key into the bloom filter.
func (bf *BloomFilter) Add(key string) {
	h1, h2 := bf.hashes(key)
	for i := 0; i < bf.numHash; i++ {
		pos := (h1 + uint64(i)*h2) % bf.size
		bf.bits[pos/64] |= 1 << (pos % 64)
	}
}

// Contains returns true if the key might be in the set, false if definitely not.
func (bf *BloomFilter) Contains(key string) bool {
	h1, h2 := bf.hashes(key)
	for i := 0; i < bf.numHash; i++ {
		pos := (h1 + uint64(i)*h2) % bf.size
		if bf.bits[pos/64]&(1<<(pos%64)) == 0 {
			return false
		}
	}
	return true
}

// hashes returns two independent hash values for double-hashing.
func (bf *BloomFilter) hashes(key string) (uint64, uint64) {
	h := xxhash.Sum64String(key)
	// Split into two 32-bit hashes and expand
	h1 := h
	h2 := xxhash.Sum64String(key + "\x00")
	return h1, h2
}

// Encode serializes the bloom filter to bytes.
func (bf *BloomFilter) Encode() ([]byte, error) {
	// Header: numHash (4 bytes) + size (8 bytes) + bits length (4 bytes)
	headerSize := 16
	dataSize := len(bf.bits) * 8
	buf := make([]byte, headerSize+dataSize)

	binary.LittleEndian.PutUint32(buf[0:4], uint32(bf.numHash))
	binary.LittleEndian.PutUint64(buf[4:12], bf.size)
	binary.LittleEndian.PutUint32(buf[12:16], uint32(len(bf.bits)))

	for i, word := range bf.bits {
		binary.LittleEndian.PutUint64(buf[headerSize+i*8:headerSize+i*8+8], word)
	}

	return buf, nil
}

// DecodeBloomFilter deserializes a bloom filter from bytes.
func DecodeBloomFilter(data []byte) (*BloomFilter, error) {
	if len(data) < 16 {
		return nil, fmt.Errorf("bloom: data too short")
	}

	numHash := int(binary.LittleEndian.Uint32(data[0:4]))
	size := binary.LittleEndian.Uint64(data[4:12])
	numWords := int(binary.LittleEndian.Uint32(data[12:16]))

	expected := 16 + numWords*8
	if len(data) < expected {
		return nil, fmt.Errorf("bloom: data truncated: need %d bytes, got %d", expected, len(data))
	}

	bits := make([]uint64, numWords)
	for i := range bits {
		bits[i] = binary.LittleEndian.Uint64(data[16+i*8 : 16+i*8+8])
	}

	return &BloomFilter{
		bits:    bits,
		numHash: numHash,
		size:    size,
	}, nil
}
