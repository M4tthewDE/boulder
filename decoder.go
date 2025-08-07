package boulder

import (
	"log"
	"math"
	"os"
)

var (
	Leb128Bytes = 0
)

type Reader struct {
	data     []byte
	bitIndex int
}

func NewReader(filePath string) Reader {
	data, err := os.ReadFile(filePath)
	if err != nil {
		panic(err)
	}

	return Reader{
		data:     data,
		bitIndex: 0,
	}

}

func (r *Reader) discard(n int) {
	r.bitIndex = r.bitIndex + n*8
}

func (r *Reader) hasRemainingData() bool {
	return r.bitIndex < len(r.data)*8
}

func (r *Reader) readBit() int {
	bit := int((r.data[int(math.Floor(float64(r.bitIndex)/8))] >> (8 - r.bitIndex%8 - 1)) & 1)
	r.bitIndex++
	return bit
}

func (r *Reader) f(n int) int {
	x := 0
	for i := 0; i < n; i++ {
		x = 2*x + r.readBit()
	}

	return x
}

func (r *Reader) leb128() int {
	value := 0
	Leb128Bytes = 0
	for i := 0; i < 8; i++ {
		lebt128_byte := r.f(8)

		value = value | (lebt128_byte&0x7f)<<(i*7)
		Leb128Bytes += 1

		if (lebt128_byte & 0x80) == 0 {
			break
		}
	}

	return value
}

type Decoder struct {
}

func NewDecoder() Decoder {
	return Decoder{}
}

func (d *Decoder) Decode(filePath string) {
	r := NewReader(filePath)

	for {
		if !r.hasRemainingData() {
			return
		}

		temporalUnitSize := r.leb128()

		log.Printf("temporalUnitSize: %d", temporalUnitSize)

		temporalUnit(&r, temporalUnitSize)
	}
}

func temporalUnit(r *Reader, size int) {
	for size > 0 {
		frameUnitSize := r.leb128()
		log.Printf("frameUnitSize: %d", frameUnitSize)

		size = size - Leb128Bytes
		frameUnit(r, frameUnitSize)
		size = size - frameUnitSize

	}
}

func frameUnit(r *Reader, size int) {
	for size > 0 {
		obuLength := r.leb128()
		log.Printf("obuLength: %d", obuLength)

		size = size - Leb128Bytes
		openBitstreamUnit(r, obuLength)
		size = size - obuLength

	}
}

func openBitstreamUnit(r *Reader, size int) {
	r.discard(size)
}

func obuHeader() {
	/*
		obuForbiddenBit := f1(r)
		obuType := f4(r)
		obuExtensionFlag := f1(r)
		obuHasSizeField := f1(r)
	*/
}
