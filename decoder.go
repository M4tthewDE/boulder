package boulder

import (
	"log"
	"math"
	"os"
)

var (
	Leb128Bytes       = 0
	OperatingPointIdc = 0
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

type DecoderResult struct {
	temporalUnitCount int
}

type Decoder struct {
}

func NewDecoder() Decoder {
	return Decoder{}
}

func (d *Decoder) Decode(filePath string) DecoderResult {
	r := NewReader(filePath)

	temporalUnitCount := 0

	for {
		if !r.hasRemainingData() {
			return DecoderResult{
				temporalUnitCount: temporalUnitCount,
			}
		}

		temporalUnitSize := r.leb128()

		log.Printf("temporalUnitSize: %d", temporalUnitSize)

		temporalUnit(&r, temporalUnitSize)
		temporalUnitCount++
	}
}

func temporalUnit(r *Reader, size int) {
	for size > 0 {
		frameUnitSize := r.leb128()

		size = size - Leb128Bytes
		frameUnit(r, frameUnitSize)
		size = size - frameUnitSize

	}
}

func frameUnit(r *Reader, size int) {
	for size > 0 {
		obuLength := r.leb128()

		size = size - Leb128Bytes
		openBitstreamUnit(r, obuLength)
		size = size - obuLength

	}
}

func openBitstreamUnit(r *Reader, size int) {
	header := obuHeader(r)

	var obuSize int
	if header.hasSizeField {
		obuSize = r.leb128()
	} else {
		obuSize = size - 1 - header.extensionFlag
	}

	if header.typ != OBU_SEQUENCE_HEADER &&
		header.typ != OBU_TEMPORAL_DELIMITER &&
		OperatingPointIdc != 0 &&
		header.extensionFlag == 1 {
		panic("todo")
	}

	if header.typ == OBU_SEQUENCE_HEADER {
		panic("todo")
	}

	r.discard(obuSize)
}

const OBU_SEQUENCE_HEADER = 1
const OBU_TEMPORAL_DELIMITER = 2

type ObuHeader struct {
	forbidden     bool
	typ           int
	hasSizeField  bool
	extensionFlag int
}

func obuHeader(r *Reader) ObuHeader {
	forbidden := r.f(1) != 0
	typ := r.f(4)
	extensionFlag := r.f(1)
	hasSizeField := r.f(1) != 0

	// reserved
	_ = r.f(1)
	if extensionFlag != 0 {
		panic("todo")
	}

	return ObuHeader{
		forbidden:     forbidden,
		typ:           typ,
		hasSizeField:  hasSizeField,
		extensionFlag: extensionFlag,
	}
}
