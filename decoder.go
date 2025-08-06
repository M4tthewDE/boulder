package boulder

import (
	"bufio"
	"io"
	"log"
	"os"
)

var (
	Leb128Bytes = 0
)

type Decoder struct {
}

func NewDecoder() Decoder {
	return Decoder{}
}

func (d *Decoder) Decode(filePath string) {
	f, err := os.Open(filePath)
	if err != nil {
		panic(err)
	}

	r := bufio.NewReader(f)

	for {
		_, err := r.Peek(1)
		if err == io.EOF {
			return
		} else if err != nil {
			panic(err)
		}

		temporalUnitSize := leb128(r)

		log.Printf("temporalUnitSize: %d", temporalUnitSize)

		temporalUnit(r, temporalUnitSize)
	}
}

func f8(r *bufio.Reader) int {
	byte, err := r.ReadByte()
	if err != nil {
		panic(err)
	}

	return int(byte)
}

func leb128(r *bufio.Reader) int {
	value := 0
	Leb128Bytes = 0
	for i := 0; i < 8; i++ {
		lebt128_byte := f8(r)

		value = value | (lebt128_byte&0x7f)<<(i*7)
		Leb128Bytes += 1

		if (lebt128_byte & 0x80) == 0 {
			break
		}
	}

	return value
}

func temporalUnit(r *bufio.Reader, size int) {
	for size > 0 {
		frameUnitSize := leb128(r)
		log.Printf("frameUnitSize: %d", frameUnitSize)

		size = size - Leb128Bytes
		frameUnit(r, frameUnitSize)
		size = size - frameUnitSize

	}
}

func frameUnit(r *bufio.Reader, size int) {
	for size > 0 {
		obuLength := leb128(r)
		log.Printf("obuLength: %d", obuLength)

		size = size - Leb128Bytes
		openBitstreamUnit(r, obuLength)
		size = size - obuLength

	}
}

func openBitstreamUnit(r *bufio.Reader, size int) {
	r.Read(make([]byte, size))
}
