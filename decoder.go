package boulder

import (
	"log"
	"math"
	"os"
)

var (
	Leb128Bytes       int
	OperatingPointIdc int
	OrderHintBits     int
	BitDepth          int
	NumPlanes         int
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

type OpenBitstreamUnit struct {
	header ObuHeader
}

type FrameUnit struct {
	obus []OpenBitstreamUnit
}

type TemporalUnit struct {
	frameUnits []FrameUnit
}

type DecoderResult struct {
	temporalUnits []TemporalUnit
}

type Decoder struct {
}

func NewDecoder() Decoder {
	return Decoder{}
}

func (d *Decoder) Decode(filePath string) DecoderResult {
	r := NewReader(filePath)

	temporalUnits := make([]TemporalUnit, 0)

	for {
		if !r.hasRemainingData() {
			return DecoderResult{
				temporalUnits: temporalUnits,
			}
		}

		temporalUnitSize := r.leb128()

		log.Printf("temporalUnitSize: %d", temporalUnitSize)

		temporalUnit := temporalUnit(&r, temporalUnitSize)
		temporalUnits = append(temporalUnits, temporalUnit)
	}
}

func temporalUnit(r *Reader, size int) TemporalUnit {
	frameUnits := make([]FrameUnit, 0)

	for size > 0 {
		frameUnitSize := r.leb128()

		size = size - Leb128Bytes
		frameUnit := frameUnit(r, frameUnitSize)
		frameUnits = append(frameUnits, frameUnit)
		size = size - frameUnitSize

	}

	return TemporalUnit{frameUnits: frameUnits}
}

func frameUnit(r *Reader, size int) FrameUnit {
	obus := make([]OpenBitstreamUnit, 0)

	for size > 0 {
		obuLength := r.leb128()

		size = size - Leb128Bytes
		obu := openBitstreamUnit(r, obuLength)
		obus = append(obus, obu)
		size = size - obuLength
	}

	return FrameUnit{obus: obus}
}

func openBitstreamUnit(r *Reader, size int) OpenBitstreamUnit {
	header := obuHeader(r)

	var obuSize int
	if header.hasSizeField {
		obuSize = r.leb128()
	} else {
		obuSize = size - 1 - header.extensionFlag
	}

	startPosition := r.bitIndex

	if header.typ != OBU_SEQUENCE_HEADER &&
		header.typ != OBU_TEMPORAL_DELIMITER &&
		OperatingPointIdc != 0 &&
		header.extensionFlag == 1 {
		panic("temporal/spatial layer")
	}

	if header.typ == OBU_SEQUENCE_HEADER {
		sequenceHeader(r)
	} else {
		r.discard(obuSize)
		return OpenBitstreamUnit{header: header}
	}

	currentPosition := r.bitIndex
	payloadBits := currentPosition - startPosition
	if obuSize > 0 && header.typ != OBU_TILE_GROUP &&
		header.typ != OBU_TILE_LIST &&
		header.typ != OBU_FRAME {
		trailingBits(r, obuSize*8-payloadBits)
	}

	return OpenBitstreamUnit{header: header}
}

func trailingBits(r *Reader, nbBits int) {
	r.f(1)
	nbBits--

	for nbBits > 0 {
		r.f(1)
		nbBits--
	}
}

const OBU_SEQUENCE_HEADER = 1
const OBU_TEMPORAL_DELIMITER = 2
const OBU_TILE_GROUP = 4
const OBU_FRAME = 6
const OBU_TILE_LIST = 8

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

	log.Printf("obu type: %d", typ)

	// reserved
	_ = r.f(1)
	if extensionFlag != 0 {
		panic("obu extension header")
	}

	return ObuHeader{
		forbidden:     forbidden,
		typ:           typ,
		hasSizeField:  hasSizeField,
		extensionFlag: extensionFlag,
	}
}

const SELECT_SCREEN_CONTENT_TOOLS = 2
const SELECT_INTEGER_MV = 2

type SequenceHeader struct {
	maxFrameWidthMinusOne           int
	maxFrameHeightMinusOne          int
	deltaFrameIdLengthMinusTwo      int
	additionalFrameIdLengthMinusTwo int
	use128x128Superblock            bool
	enableFilterIntra               bool
	enableIntraEdgeFilter           bool
	enableInterIntraCompound        bool
	enableMaskedCompound            bool
	enableWarpedMotion              bool
	enableDualFilter                bool
	enableJntComp                   bool
	enableRefFrameMvs               bool
	seqForceIntegerMv               int
	enableSuperres                  bool
	enableCdef                      bool
	enableRestoration               bool
	colorConfig                     ColorConfig
}

func sequenceHeader(r *Reader) SequenceHeader {
	seqProfile := r.f(3)
	_ = r.f(1) != 0
	reducedStillPictureHeader := r.f(1) != 0

	var operatingPointIdc []int

	if reducedStillPictureHeader {
		panic("reducedStillPictureHeader")
	} else {
		timingInfoPresent := r.f(1) != 0
		if timingInfoPresent {
			_ = timingInfo(r)
		}

		var decoderModelInf DecoderModelInfo

		decoderModelInfoPresent := r.f(1) != 0
		if decoderModelInfoPresent {
			decoderModelInf = decoderModelInfo(r)
		}

		initialDisplayDelayPresentFlag := r.f(1) != 0
		operatingPointsCountMinusOne := r.f(5)

		operatingPointIdc = make([]int, operatingPointsCountMinusOne+1)
		seqLevelIdx := make([]int, operatingPointsCountMinusOne+1)
		seqTier := make([]int, operatingPointsCountMinusOne+1)
		decoderModelInfoPresentForThisOp := make([]bool, operatingPointsCountMinusOne+1)
		operatingParamters := make([]OperatingParametersInfo, operatingPointsCountMinusOne+1)
		initialDisplayDelayPresentForThisOp := make([]bool, operatingPointsCountMinusOne+1)
		initialDisplayDelayMinusOne := make([]int, operatingPointsCountMinusOne+1)

		for i := 0; i <= operatingPointsCountMinusOne; i++ {
			operatingPointIdc[i] = r.f(12)
			seqLevelIdx[i] = r.f(5)

			if seqLevelIdx[i] > 7 {
				seqTier[i] = r.f(1)
			} else {
				seqTier[i] = 0
			}

			if decoderModelInfoPresent {
				decoderModelInfoPresentForThisOp[i] = r.f(1) != 0
				if decoderModelInfoPresentForThisOp[i] {
					operatingParamters[i] = operatingParametersInfo(r, decoderModelInf.bufferDelayLengthMinusOne+1)
				}
			} else {
				decoderModelInfoPresentForThisOp[i] = false
			}

			if initialDisplayDelayPresentFlag {
				initialDisplayDelayPresentForThisOp[i] = r.f(1) != 0
				if initialDisplayDelayPresentForThisOp[i] {
					initialDisplayDelayMinusOne[i] = r.f(4)
				}
			}
		}
	}

	OperatingPointIdc = operatingPointIdc[chooseOperatingPoint()]

	frameWidthBitsMinusOne := r.f(4)
	frameHeightBitsMinusOne := r.f(4)

	maxFrameWidthMinusOne := r.f(frameWidthBitsMinusOne + 1)
	maxFrameHeightMinusOne := r.f(frameHeightBitsMinusOne + 1)

	frameIdNumbersPresentFlag := false
	if !reducedStillPictureHeader {
		frameIdNumbersPresentFlag = r.f(1) != 0
	}

	var deltaFrameIdLengthMinusTwo int
	var additionalFrameIdLengthMinusOne int
	if frameIdNumbersPresentFlag {
		deltaFrameIdLengthMinusTwo = r.f(4)
		additionalFrameIdLengthMinusOne = r.f(3)
	}

	use128x128Superblock := r.f(1) != 0
	enableFilterIntra := r.f(1) != 0
	enableIntraEdgeFilter := r.f(1) != 0

	var enableInterIntraCompound bool
	var enableMaskedCompound bool
	var enableWarpedMotion bool
	var enableDualFilter bool
	var enableJntComp bool
	var enableRefFrameMvs bool
	var seqForceIntegerMv int

	if reducedStillPictureHeader {
		panic("reduced still picture header")
	} else {
		enableInterIntraCompound = r.f(1) != 0
		enableMaskedCompound = r.f(1) != 0
		enableWarpedMotion = r.f(1) != 0
		enableDualFilter = r.f(1) != 0
		enableOrderHint := r.f(1) != 0

		enableJntComp = false
		enableRefFrameMvs = false

		if enableOrderHint {
			enableJntComp = r.f(1) != 0
			enableRefFrameMvs = r.f(1) != 0
		}

		seqForceScreenContentTools := SELECT_SCREEN_CONTENT_TOOLS
		if r.f(1) == 0 {
			seqForceScreenContentTools = r.f(1)
		}

		seqForceIntegerMv = SELECT_INTEGER_MV
		if seqForceScreenContentTools > 0 {
			if r.f(1) == 0 {
				seqForceIntegerMv = r.f(1)
			}
		}

		if enableOrderHint {
			OrderHintBits = r.f(3) + 1
		} else {
			OrderHintBits = 0
		}
	}

	enableSuperres := r.f(1) != 0
	enableCdef := r.f(1) != 0
	enableRestoration := r.f(1) != 0
	colorConfig := colorConfig(r, seqProfile)

	return SequenceHeader{
		maxFrameWidthMinusOne:           maxFrameWidthMinusOne,
		maxFrameHeightMinusOne:          maxFrameHeightMinusOne,
		deltaFrameIdLengthMinusTwo:      deltaFrameIdLengthMinusTwo,
		additionalFrameIdLengthMinusTwo: additionalFrameIdLengthMinusOne,
		use128x128Superblock:            use128x128Superblock,
		enableFilterIntra:               enableFilterIntra,
		enableIntraEdgeFilter:           enableIntraEdgeFilter,
		enableInterIntraCompound:        enableInterIntraCompound,
		enableMaskedCompound:            enableMaskedCompound,
		enableWarpedMotion:              enableWarpedMotion,
		enableDualFilter:                enableDualFilter,
		enableJntComp:                   enableJntComp,
		enableRefFrameMvs:               enableRefFrameMvs,
		seqForceIntegerMv:               seqForceIntegerMv,
		enableSuperres:                  enableSuperres,
		enableCdef:                      enableCdef,
		enableRestoration:               enableRestoration,
		colorConfig:                     colorConfig,
	}
}

type TimingInfo struct {
	numUnitsInDisplayTick      int
	timeScale                  int
	equalPictureInterval       bool
	numTicksPerPictureMinusOne int
}

func timingInfo(r *Reader) TimingInfo {
	numUnitsInDisplayTick := r.f(32)
	timeScale := r.f(32)
	equalPictureInterval := r.f(1) != 0
	if equalPictureInterval {
		panic("equal picture interval")
	}

	return TimingInfo{
		numUnitsInDisplayTick:      numUnitsInDisplayTick,
		timeScale:                  timeScale,
		equalPictureInterval:       equalPictureInterval,
		numTicksPerPictureMinusOne: 0,
	}
}

type DecoderModelInfo struct {
	bufferDelayLengthMinusOne           int
	numUnitsInDecodingTick              int
	bufferRemovalTimeLengthMinusOne     int
	framePresentationTimeLengthMinusOne int
}

func decoderModelInfo(r *Reader) DecoderModelInfo {
	return DecoderModelInfo{
		bufferDelayLengthMinusOne:           r.f(5),
		numUnitsInDecodingTick:              r.f(32),
		bufferRemovalTimeLengthMinusOne:     r.f(5),
		framePresentationTimeLengthMinusOne: r.f(5),
	}
}

type OperatingParametersInfo struct {
	decoderBufferDelay int
	encoderBufferDelay int
	lowDelayModeFlag   bool
}

func operatingParametersInfo(r *Reader, bufferDelayLength int) OperatingParametersInfo {
	return OperatingParametersInfo{
		decoderBufferDelay: r.f(bufferDelayLength),
		encoderBufferDelay: r.f(bufferDelayLength),
		lowDelayModeFlag:   r.f(1) != 0,
	}
}

func chooseOperatingPoint() int {
	return 0
}

const CP_BT_709 = 1
const CP_UNSPECIFIED = 2

const TC_UNSPECIFIED = 2
const TC_SRGB = 13

const MC_IDENTITY = 0
const MC_UNSPECIFIED = 2
const CSP_UNKNOWN = 0

type ColorConfig struct {
	colorPrimaries          int
	transferCharacteristics int
	matrixCoefficients      int
	colorRange              int
	subsamplingX            int
	subsamplingY            int
	chromaSamplePosition    int
	separateUvDeltaQ        bool
}

func colorConfig(r *Reader, seqProfile int) ColorConfig {
	highBitdepth := r.f(1) != 0

	if seqProfile == 2 && highBitdepth {
		if r.f(1) != 0 {
			BitDepth = 12
		} else {
			BitDepth = 10
		}
	} else if seqProfile <= 2 {
		if r.f(1) != 0 {
			BitDepth = 10
		} else {
			BitDepth = 8
		}
	}

	monoChrome := false
	if seqProfile != 1 {
		monoChrome = r.f(1) != 0
	}

	if monoChrome {
		NumPlanes = 1
	} else {
		NumPlanes = 3
	}

	colorPrimaries := CP_UNSPECIFIED
	transferCharacteristics := TC_UNSPECIFIED
	matrixCoefficients := MC_UNSPECIFIED

	if r.f(1) == 0 {
		colorPrimaries = r.f(8)
		transferCharacteristics = r.f(8)
		matrixCoefficients = r.f(8)
	}

	var colorRange int
	var subsamplingX int
	var subsamplingY int
	var chromeSamplePosition int

	if monoChrome {
		colorRange := r.f(1)
		return ColorConfig{
			colorPrimaries:          colorPrimaries,
			transferCharacteristics: transferCharacteristics,
			matrixCoefficients:      matrixCoefficients,
			colorRange:              colorRange,
			subsamplingX:            1,
			subsamplingY:            1,
			chromaSamplePosition:    CSP_UNKNOWN,
			separateUvDeltaQ:        false,
		}
	} else if colorPrimaries == CP_BT_709 &&
		transferCharacteristics == TC_SRGB &&
		matrixCoefficients == MC_IDENTITY {
		colorRange = 0
		subsamplingX = 0
		subsamplingY = 0
	} else {
		colorRange = r.f(1)
		if seqProfile == 0 {
			subsamplingX = 1
			subsamplingY = 1
		} else if seqProfile == 1 {
			subsamplingX = 0
			subsamplingY = 0
		} else {
			if BitDepth == 12 {
				subsamplingX = r.f(1)
				subsamplingY = 0

				if subsamplingX != 0 {
					subsamplingY = r.f(1)
				}
			} else {
				subsamplingX = 1
				subsamplingY = 0
			}
		}

		if subsamplingX != 0 && subsamplingY != 0 {
			chromeSamplePosition = r.f(2)
		}
	}

	return ColorConfig{
		colorPrimaries:          colorPrimaries,
		transferCharacteristics: transferCharacteristics,
		matrixCoefficients:      matrixCoefficients,
		colorRange:              colorRange,
		subsamplingX:            subsamplingX,
		subsamplingY:            subsamplingY,
		chromaSamplePosition:    chromeSamplePosition,
		separateUvDeltaQ:        r.f(1) != 0,
	}
}
