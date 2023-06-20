package vint64

import (
	"bytes"
	"math"
	"testing"
	"time"

	"github.com/ericlagergren/testutil"
	"golang.org/x/exp/rand"
)

func seed() uint64 {
	return uint64(time.Now().UnixNano())
}

func TestIssue1(t *testing.T) {
	data := make([]byte, 480)
	for i := range data {
		data[i] = 'A'
	}
	got := Append(nil, uint64(len(data)))
	got = append(got, data...)
	v, err := Decode(got)
	if err != nil {
		t.Fatal(err)
	}
	if v != uint64(len(data)) {
		t.Fatalf("got %d, expected %d", v, len(data))
	}
}

func TestEncode(t *testing.T) {
	for i, tc := range []struct {
		v    uint64
		want []byte
	}{
		{0, []byte{1}},
		{0x0f0f, []byte{0x3e, 0x3c}},
		{0x0f0f_f0f0, []byte{0x08, 0x0f, 0xff, 0xf0}},
		{0x0f0f_f0f0_0f0f, []byte{0xc0, 0x87, 0x07, 0x78, 0xf8, 0x87, 0x07}},
		{0x0f0f_f0f0_0f0f_f0f0, []byte{0x00, 0xf0, 0xf0, 0x0f, 0x0f, 0xf0, 0xf0, 0x0f, 0x0f}},
		{math.MaxUint64, []byte{0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
	} {
		var b [MaxLen]byte
		n := Encode(&b, tc.v)
		got := b[:n]
		if !bytes.Equal(got, tc.want) {
			t.Fatalf("#%d: got %#v, expected %#v", i, got, tc.want)
		}
		if got := Append(nil, tc.v); !bytes.Equal(got, tc.want) {
			t.Fatalf("#%d: got %#v, expected %#v", i, got, tc.want)
		}

		v, err := Decode(got)
		if err != nil {
			t.Fatalf("#%d: %v", i, err)
		}
		if v != tc.v {
			t.Fatalf("#%d: got %#x, expected %#x", i, v, tc.v)
		}
		v, err = Read(bytes.NewReader(got))
		if err != nil {
			t.Fatalf("#%d: %v", i, err)
		}
		if v != tc.v {
			t.Fatalf("#%d: got %#x, expected %#x", i, v, tc.v)
		}
	}
}

func TestEncodedLen(t *testing.T) {
	for _, tc := range []struct {
		lo, hi, want int
	}{
		{0, 7, 9},
		{8, 14, 8},
		{15, 21, 7},
		{22, 28, 6},
		{29, 35, 5},
		{36, 42, 4},
		{43, 49, 3},
		{50, 56, 2},
		{57, 64, 1},
	} {
		for i := tc.lo; i <= tc.hi; i++ {
			v := uint64((1 << (64 - i)) - 1)
			got := EncodedLen(v)
			if got != tc.want {
				t.Fatalf("#%d: got %d, expected %d", i, got, tc.want)
			}
		}
	}
}

func TestDecodedLen(t *testing.T) {
	for i := 0; i < 256; i++ {
		n := DecodedLen(byte(i))
		if n < 0 || n > 9 {
			t.Fatalf("got %d", n)
		}
	}
}

func TestInlining(t *testing.T) {
	want := []string{
		"Append",
		"Decode",
		"DecodedLen",
		"Encode",
		"EncodedLen",
		"Unzigzag",
		"Zigzag",
		"encLen",
	}
	testutil.TestInlining(t, "github.com/ericlagergren/vint64", want...)
}

func TestAllocs(t *testing.T) {
	rng := rand.New(rand.NewSource(seed()))
	var b [MaxLen]byte
	n := Encode(&b, rng.Uint64())
	r := bytes.NewReader(b[:n])

	test := func(t *testing.T, name string, fn func()) {
		t.Helper()
		n := int(testing.AllocsPerRun(100, fn))
		if n != 0 {
			t.Fatalf("got %d, expected %d", n, 0)
		}
	}
	test(t, "Read", func() {
		sink.uint64, sink.err = Read(r)
	})
	test(t, "Decode", func() {
		sink.uint64, sink.err = Decode(b[:n])
	})
	test(t, "Encode", func() {
		sink.int = Encode(&b, rng.Uint64())
	})
	test(t, "Append", func() {
		sink.bytes = Append(sink.bytes[:0], rng.Uint64())
	})
}

var sink struct {
	buf    [MaxLen]byte
	int    int
	uint64 uint64
	bytes  []byte
	err    error
}

func BenchmarkEncode(b *testing.B) {
	rng := rand.New(rand.NewSource(seed()))
	// s is a power of two array so that len(s) gets compiled
	// into a mask, which ~free compared to modulo.
	var s [1024]uint64
	for i := range s {
		s[i] = rng.Uint64()
	}
	p := &sink.buf
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sink.int = Encode(p, s[i%len(s)])
	}
}

func BenchmarkAppend(b *testing.B) {
	rng := rand.New(rand.NewSource(seed()))
	// s is a power of two array so that len(s) gets compiled
	// into a mask, which ~free compared to modulo.
	var s [1024]uint64
	for i := range s {
		s[i] = rng.Uint64()
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sink.bytes = Append(sink.bytes[:0], s[i%len(s)])
	}
}

func BenchmarkDecode(b *testing.B) {
	rng := rand.New(rand.NewSource(seed()))
	// s is a power of two array so that len(s) gets compiled
	// into a mask, which ~free compared to modulo.
	var s [1024][]byte
	for i := range s {
		var b [MaxLen]byte
		n := Encode(&b, rng.Uint64())
		s[i] = b[:n]
	}
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		v, err := Decode(s[i%len(s)])
		if err != nil {
			b.Fatal(err)
		}
		sink.uint64 = v
	}
}

func BenchmarkRead(b *testing.B) {
	rng := rand.New(rand.NewSource(seed()))
	// s is a power of two array so that len(s) gets compiled
	// into a mask, which ~free compared to modulo.
	var s [1024][]byte
	for i := range s {
		var b [MaxLen]byte
		n := Encode(&b, rng.Uint64())
		s[i] = b[:n]
	}
	var r bytes.Reader
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r.Reset(s[i%len(s)])
		v, err := Read(&r)
		if err != nil {
			b.Fatal(err)
		}
		sink.uint64 = v
	}
}
