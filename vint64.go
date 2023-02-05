// Package vint64 implements the [vint64] variable length integer
// encoding
//
// [vint64]: https://docs.rs/vint64/latest/vint64/
package vint64

import (
	"encoding/binary"
	"errors"
	"io"
	"math/bits"
)

// ErrLeadingZeros is returned by [Decode] when the encoded
// integer contains unnecessary leading zeros.
var ErrLeadingZeros = errors.New("vint: encoded integer contains leading zeros")

// MaxLen is the maximum number of bytes required to encode an
// integer.
const MaxLen = 9

// Zigzag encodes a signed integer as an unsigned integer.
func Zigzag(v int64) uint64 {
	return uint64(v<<1) ^ uint64(v>>63)
}

// Unzigzag decodes the unsigned integer as a signed integer.
func Unzigzag(v uint64) int64 {
	return int64(v>>1) ^ int64(v)<<63>>63
}

// Read parses an integer from b.
//
// To decode a signed integer, convert the result with
// [Unzigzag].
func Read(r io.ByteReader) (uint64, error) {
	c, err := r.ReadByte()
	if err != nil {
		return 0, err
	}
	n := DecodedLen(c)
	b := make([]byte, MaxLen)
	b[0] = c
	_ = b[n-1] // bounds check hint for the compiler
	for i := 1; i < n; i++ {
		var err error
		b[i], err = r.ReadByte()
		if err != nil {
			return 0, err
		}
	}
	return Decode(b[:n])
}

// Decode parses an integer from b.
//
// The number of bytes read can be determined with [DecodedLen].
//
// To decode a signed integer, convert the result with
// [Unzigzag].
func Decode(b []byte) (v uint64, err error) {
	if len(b) == 0 || bits.TrailingZeros8(b[0]) >= len(b) {
		// Combine these cases to shrink the resulting assembly.
		// If the n >= len(b) check is performed separately, the
		// compiler generates duplicate assembly for the
		// identical return statements.
		return 0, io.ErrUnexpectedEOF
	}
	// Manually inline the call to DecodedLen so that the
	// compiler can inline Decode.
	//
	// Writing n >= MaxLen instead of n == MaxLen lets the
	// compiler to provde that the right shift by n in the "else"
	// branch is less than 64, and avoids generating code to
	// reduce n mod 64.
	n := uint(bits.TrailingZeros8(b[0]))
	if n >= 8 {
		v = binary.LittleEndian.Uint64(b[1:])
	} else {
		e := make([]byte, 8)
		copy(e, b)
		v = binary.LittleEndian.Uint64(e) >> (n + 1)
	}
	if n != 0 && v < 1<<(7*n) {
		return 0, ErrLeadingZeros
	}
	return v, nil
}

// DecodedLen returns the number of bytes in the encoded integer.
//
// The result will always be in [0, 9].
func DecodedLen(b byte) int {
	return bits.TrailingZeros8(b) + 1
}

// Encode writes v to b, returning the number of bytes written.
//
// To encode a signed integer, convert the input with [Zigzag].
func Encode(b *[MaxLen]byte, v uint64) int {
	// Using encLen and checking n >= 8 helps the compiler prove
	// two things:
	//
	// 1. That the left shift by n cannot panic (because n is
	//    unsigned).
	// 2. That the shift count n does not need to be reduced mod
	//    64 (x86 only).
	n := encLen(v)
	if n >= 8 {
		binary.LittleEndian.PutUint64(b[1:], v)
	} else {
		binary.LittleEndian.PutUint64(b[:], (v<<1|1)<<n)
	}
	return int(n + 1)
}

// EncodedLen returns the number of bytes necessary to encode v.
//
// The result will always be in [0, 9].
func EncodedLen(v uint64) int {
	return int(encLen(v) + 1)
}

func encLen(v uint64) uint {
	n := bits.LeadingZeros64(v)
	if n == 0 {
		n = 1
	}
	return uint((63 - n) / 7)
}
