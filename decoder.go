package boulder

import (
	"log"
	"math"
	"os"
)

const MAX_SEGMENTS = 8
const SEG_LVL_MAX = 8

var (
	Leb128Bytes       int
	OperatingPointIdc int
	OrderHintBits     int
	BitDepth          int
	NumPlanes         int
	SeenFrameHeader   bool
	FrameIsIntra      bool
	RefFrameId        = make([]int, NUM_REF_FRAMES)
	RefValid          = make([]int, NUM_REF_FRAMES)
	RefOrderHint      = make([]int, NUM_REF_FRAMES)
	OrderHints        = make([]int, REFS_PER_FRAME+LAST_FRAME)
	PrevFrameId       int
	OrderHint         int
	sh                SequenceHeader
	currentFrameId    = 0
	temporalId        = 0
	spatialId         = 0
	FrameWidth        int
	FrameHeight       int
	SuperresDenom     int
	UpscaledWidth     int
	MiCols            int
	MiRows            int
	RenderWidth       int
	RenderHeight      int
	FeatureData       [SEG_LVL_MAX][MAX_SEGMENTS]int
	PrevSegmentIds    [][]int
	GmType            []int
	PrevGmParams      [][]int
	TileColsLog2      int
	TileCols          int
	TileRowsLog2      int
	TileRows          int
	MiColStarts       []int
	MiRowStarts       []int
	TileSizeBytes     int
	DeltaQUDc         int
	DeltaQUAc         int
	DeltaQYDc         int
	DeltaQVAc         int
	DeltaQVDc         int
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

	if value > (1<<32)-1 {
		panic("invalid leb128 value")
	}

	return value
}

func (r *Reader) su(n int) int {
	value := r.f(n)
	signMask := 1 << (n - 1)
	if (value & signMask) != 0 {
		return value - 2*signMask
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
		sh = sequenceHeader(r)
	} else if header.typ == OBU_TEMPORAL_DELIMITER {
		SeenFrameHeader = false
		r.discard(obuSize)
		return OpenBitstreamUnit{header: header}
	} else if header.typ == OBU_FRAME_HEADER {
		frameHeader(r, sh)
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
const OBU_FRAME_HEADER = 3
const OBU_TILE_GROUP = 4
const OBU_FRAME = 6
const OBU_TILE_LIST = 8
const OBU_PADDING = 15

type ObuHeader struct {
	forbidden     bool
	typ           int
	hasSizeField  bool
	extensionFlag int
}

func obuHeader(r *Reader) ObuHeader {
	forbidden := r.f(1) != 0

	if forbidden {
		panic("forbidden bit must be 0")
	}

	typ := r.f(4)
	extensionFlag := r.f(1)
	hasSizeField := r.f(1) != 0

	log.Printf("obu type: %d", typ)

	// reserved
	reserved := r.f(1) != 0

	if reserved {
		panic("reserved bit must be 0")
	}

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
	maxFrameWidthMinusOne            int
	maxFrameHeightMinusOne           int
	deltaFrameIdLengthMinusTwo       int
	additionalFrameIdLengthMinusTwo  int
	use128x128Superblock             bool
	enableFilterIntra                bool
	enableIntraEdgeFilter            bool
	enableInterIntraCompound         bool
	enableMaskedCompound             bool
	enableWarpedMotion               bool
	enableDualFilter                 bool
	enableJntComp                    bool
	enableRefFrameMvs                bool
	seqForceIntegerMv                int
	enableSuperres                   bool
	enableCdef                       bool
	enableRestoration                bool
	colorConfig                      ColorConfig
	frameIdNumbersPresentFlag        bool
	reducedStillPictureHeader        bool
	decoderModelInfoPresentFlag      bool
	timingInfo                       TimingInfo
	seqForceScreenContentTools       int
	decoderModelInfo                 DecoderModelInfo
	operatingPointsCountMinusOne     int
	decoderModelInfoPresentForThisOp []bool
	operatingPointIdc                []int
	enableOrderHint                  bool
	frameWidthBitsMinusOne           int
	frameHeightBitsMinusOne          int
	stillPicture                     bool
}

func sequenceHeader(r *Reader) SequenceHeader {
	seqProfile := r.f(3)
	log.Printf("seqProfile: %d", seqProfile)

	if seqProfile > 2 {
		panic("invalid seqProfile")
	}

	stillPicture := r.f(1) != 0
	reducedStillPictureHeader := r.f(1) != 0

	var operatingPointIdc []int
	var decoderModelInfoPresentFlag bool
	var timingInf TimingInfo
	var decoderModelInf DecoderModelInfo
	var operatingPointsCountMinusOne int
	decoderModelInfoPresentForThisOp := make([]bool, operatingPointsCountMinusOne+1)

	if reducedStillPictureHeader {
		panic("reducedStillPictureHeader")
	} else {
		timingInfoPresentFlag := r.f(1) != 0
		if timingInfoPresentFlag {
			timingInf = timingInfo(r)
		}

		decoderModelInfoPresentFlag = r.f(1) != 0
		if decoderModelInfoPresentFlag {
			decoderModelInf = decoderModelInfo(r)
		}

		initialDisplayDelayPresentFlag := r.f(1) != 0
		operatingPointsCountMinusOne = r.f(5)

		operatingPointIdc = make([]int, operatingPointsCountMinusOne+1)
		seqLevelIdx := make([]int, operatingPointsCountMinusOne+1)
		seqTier := make([]int, operatingPointsCountMinusOne+1)
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

			if decoderModelInfoPresentFlag {
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
	var seqForceScreenContentTools int
	var enableOrderHint bool

	if reducedStillPictureHeader {
		panic("reduced still picture header")
	} else {
		enableInterIntraCompound = r.f(1) != 0
		enableMaskedCompound = r.f(1) != 0
		enableWarpedMotion = r.f(1) != 0
		enableDualFilter = r.f(1) != 0
		enableOrderHint = r.f(1) != 0

		enableJntComp = false
		enableRefFrameMvs = false

		if enableOrderHint {
			enableJntComp = r.f(1) != 0
			enableRefFrameMvs = r.f(1) != 0
		}

		seqForceScreenContentTools = SELECT_SCREEN_CONTENT_TOOLS
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
		maxFrameWidthMinusOne:            maxFrameWidthMinusOne,
		maxFrameHeightMinusOne:           maxFrameHeightMinusOne,
		deltaFrameIdLengthMinusTwo:       deltaFrameIdLengthMinusTwo,
		additionalFrameIdLengthMinusTwo:  additionalFrameIdLengthMinusOne,
		use128x128Superblock:             use128x128Superblock,
		enableFilterIntra:                enableFilterIntra,
		enableIntraEdgeFilter:            enableIntraEdgeFilter,
		enableInterIntraCompound:         enableInterIntraCompound,
		enableMaskedCompound:             enableMaskedCompound,
		enableWarpedMotion:               enableWarpedMotion,
		enableDualFilter:                 enableDualFilter,
		enableJntComp:                    enableJntComp,
		enableRefFrameMvs:                enableRefFrameMvs,
		seqForceIntegerMv:                seqForceIntegerMv,
		enableSuperres:                   enableSuperres,
		enableCdef:                       enableCdef,
		enableRestoration:                enableRestoration,
		colorConfig:                      colorConfig,
		frameIdNumbersPresentFlag:        frameIdNumbersPresentFlag,
		reducedStillPictureHeader:        reducedStillPictureHeader,
		decoderModelInfoPresentFlag:      decoderModelInfoPresentFlag,
		timingInfo:                       timingInf,
		seqForceScreenContentTools:       seqForceScreenContentTools,
		decoderModelInfo:                 decoderModelInf,
		operatingPointsCountMinusOne:     operatingPointsCountMinusOne,
		decoderModelInfoPresentForThisOp: decoderModelInfoPresentForThisOp,
		operatingPointIdc:                operatingPointIdc,
		enableOrderHint:                  enableOrderHint,
		frameWidthBitsMinusOne:           frameWidthBitsMinusOne,
		frameHeightBitsMinusOne:          frameHeightBitsMinusOne,
		stillPicture:                     stillPicture,
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

	log.Printf("BitDepth: %d", BitDepth)

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

func frameHeader(r *Reader, sh SequenceHeader) {
	if SeenFrameHeader {
		panic("frame header copy")
	} else {
		SeenFrameHeader = true
		_ = uncompressedHeader(r, sh)
		panic("frame header")
	}
}

const NUM_REF_FRAMES = 8
const KEY_FRAME = 0
const INTRA_ONLY_FRAME = 2
const SWITCH_FRAME = 3

const REFS_PER_FRAME = 7

const INTRA_FRAME = 0
const LAST_FRAME = 1
const LAST2_FRAME = 2
const LAST3_FRAME = 3
const GOLDEN_FRAME = 4
const BWDREF_FRAME = 5
const ALTREF2_FRAME = 6
const ALTREF_FRAME = 7

const PRIMARY_REF_NONE = 7

type UncompressedHeader struct{}

func uncompressedHeader(r *Reader, sh SequenceHeader) UncompressedHeader {
	var idLen int
	if sh.frameIdNumbersPresentFlag {
		idLen = (sh.additionalFrameIdLengthMinusTwo + sh.deltaFrameIdLengthMinusTwo + 3)
	}

	_ = (1 << NUM_REF_FRAMES) - 1

	showExistingFrame := false
	frameType := KEY_FRAME
	FrameIsIntra = true
	showFrame := true
	showableFrame := false
	var errorResilientMode bool
	var framePresentationTime int

	allFrames := (1 << NUM_REF_FRAMES) - 1

	if !sh.reducedStillPictureHeader {
		showExistingFrame = r.f(1) != 0
		if showExistingFrame {
			panic("show existing frame")
		}

		frameType = r.f(2)
		FrameIsIntra = frameType == INTRA_ONLY_FRAME || frameType == KEY_FRAME
		showFrame = r.f(1) != 0
		if showFrame && sh.decoderModelInfoPresentFlag && !sh.timingInfo.equalPictureInterval {
			framePresentationTime = r.f(sh.decoderModelInfo.framePresentationTimeLengthMinusOne + 1)
		}

		if showFrame {
			showableFrame = frameType != KEY_FRAME
		} else {
			showableFrame = r.f(1) != 0
		}

		if frameType == SWITCH_FRAME || (frameType == KEY_FRAME && showFrame) {
			errorResilientMode = true
		} else {
			errorResilientMode = r.f(1) != 0
		}
	}

	if frameType == KEY_FRAME && showFrame {
		for i := 0; i < NUM_REF_FRAMES; i++ {
			RefValid[i] = 0
			RefOrderHint[i] = 0
		}
		for i := 0; i < REFS_PER_FRAME; i++ {
			OrderHints[LAST_FRAME+1] = 0
		}
	}

	disableCdfUpdate := r.f(1) != 0

	var allowScreenContentTools bool
	if sh.seqForceScreenContentTools == SELECT_SCREEN_CONTENT_TOOLS {
		allowScreenContentTools = r.f(1) != 0
	} else {
		allowScreenContentTools = sh.seqForceScreenContentTools != 0
	}

	var forceIntegerMv bool
	if allowScreenContentTools {
		if sh.seqForceIntegerMv == SELECT_INTEGER_MV {
			forceIntegerMv = r.f(1) != 0
		} else {
			forceIntegerMv = sh.seqForceIntegerMv != 0
		}
	} else {
		forceIntegerMv = false
	}

	if FrameIsIntra {
		forceIntegerMv = true
	}

	if sh.frameIdNumbersPresentFlag {
		PrevFrameId = currentFrameId
		currentFrameId = r.f(idLen)
		markRefRames(idLen)
	} else {
		currentFrameId = 0
	}

	var frameSizeOverrideFlag bool
	if frameType == SWITCH_FRAME {
		frameSizeOverrideFlag = true
	} else if sh.reducedStillPictureHeader {
		frameSizeOverrideFlag = false
	} else {
		frameSizeOverrideFlag = r.f(1) != 0
	}

	OrderHint = r.f(OrderHintBits)

	var primaryRefFrame int
	if FrameIsIntra || errorResilientMode {
		primaryRefFrame = PRIMARY_REF_NONE
	} else {
		primaryRefFrame = r.f(3)
	}

	bufferRemovalTime := make([]int, sh.operatingPointsCountMinusOne+1)

	if sh.decoderModelInfoPresentFlag {
		if r.f(1) != 0 {
			for opNum := 0; opNum <= sh.operatingPointsCountMinusOne; opNum++ {
				if sh.decoderModelInfoPresentForThisOp[opNum] {
					opPtIdc := sh.operatingPointIdc[opNum]
					inTemporalLayer := ((opPtIdc >> temporalId) & 1) != 0
					inSpatialLayer := ((opPtIdc >> (spatialId + 8)) & 1) != 0

					if opPtIdc == 0 || (inTemporalLayer && inSpatialLayer) {
						bufferRemovalTime[opNum] = r.f(sh.decoderModelInfo.bufferRemovalTimeLengthMinusOne + 1)
					}
				}
			}
		}
	}

	useRefFrameMvs := false
	var refreshFrameFlags int
	if frameType == SWITCH_FRAME || (frameType == KEY_FRAME && showFrame) {
		refreshFrameFlags = allFrames
	} else {
		refreshFrameFlags = r.f(8)
	}

	if !FrameIsIntra || refreshFrameFlags != allFrames {
		if errorResilientMode && sh.enableOrderHint {
			panic("todo")
		}
	}

	var allowIntrabc bool
	var frameRefsShortSignaling bool

	if FrameIsIntra {
		frameSize(r, frameSizeOverrideFlag)
		renderSize(r)
		if allowScreenContentTools && UpscaledWidth == FrameWidth {
			allowIntrabc = r.f(1) != 0
		}
	} else {
		if !sh.enableOrderHint {
			frameRefsShortSignaling = false
		} else {
			frameRefsShortSignaling = r.f(1) != 0
			if !frameRefsShortSignaling {
				panic("todo")
			}
		}

		panic("todo")
	}

	var disableFrameEndUpdateCdf bool
	if sh.reducedStillPictureHeader || disableCdfUpdate {
		disableFrameEndUpdateCdf = true
	} else {
		disableFrameEndUpdateCdf = r.f(1) != 0
	}

	var loopFilterDeltaEnabled bool
	var loopFilterRefDeltas []int
	var loopFilterModeDeltas []int

	if primaryRefFrame == PRIMARY_REF_NONE {
		log.Println("todo: init_non_coeff_cdfs()")
		setupPastIndependence()
		loopFilterDeltaEnabled = true

		loopFilterRefDeltas = make([]int, 8)
		loopFilterRefDeltas[INTRA_FRAME] = 1
		loopFilterRefDeltas[LAST_FRAME] = 0
		loopFilterRefDeltas[LAST2_FRAME] = 0
		loopFilterRefDeltas[LAST3_FRAME] = 0
		loopFilterRefDeltas[BWDREF_FRAME] = 0
		loopFilterRefDeltas[GOLDEN_FRAME] = -1
		loopFilterRefDeltas[ALTREF_FRAME] = -1
		loopFilterRefDeltas[ALTREF2_FRAME] = -1

		loopFilterModeDeltas = make([]int, 2)
		loopFilterModeDeltas[0] = 0
		loopFilterModeDeltas[1] = 0
	} else {
		panic("todo")
	}

	if useRefFrameMvs {
		panic("todo")
	}
	contextUpdateTileId := tileInfo(r)
	quantizationParams := quantizationParams(r)

	panic("uncompressed header")

	log.Println(showableFrame, forceIntegerMv, primaryRefFrame, framePresentationTime, allowIntrabc, disableFrameEndUpdateCdf, loopFilterDeltaEnabled, contextUpdateTileId, quantizationParams)
	return UncompressedHeader{}
}

type QuantizationParams struct {
	baseQIdx int
	qmY      int
	qmU      int
	qmV      int
}

func quantizationParams(r *Reader) QuantizationParams {
	baseQIdx := r.f(8)
	DeltaQYDc = readDeltaQ(r)

	DeltaQUDc = 0
	DeltaQUAc = 0
	DeltaQVDc = 0
	DeltaQVAc = 0

	if NumPlanes > 1 {
		diffUvDelta := false
		if sh.colorConfig.separateUvDeltaQ {
			diffUvDelta = r.f(1) != 0
		}

		DeltaQUDc = readDeltaQ(r)
		DeltaQUAc = readDeltaQ(r)

		if diffUvDelta {
			DeltaQVDc = readDeltaQ(r)
			DeltaQVAc = readDeltaQ(r)
		} else {
			DeltaQVDc = DeltaQUDc
			DeltaQVAc = DeltaQUAc
		}
	}

	var qmY int
	var qmU int
	var qmV int

	usingQMatrix := r.f(1) != 0
	if usingQMatrix {
		qmY = r.f(4)
		qmU = r.f(4)

		if !sh.colorConfig.separateUvDeltaQ {
			qmV = qmU
		} else {
			qmV = r.f(4)
		}
	}

	return QuantizationParams{baseQIdx: baseQIdx, qmY: qmY, qmU: qmU, qmV: qmV}

}

func readDeltaQ(r *Reader) int {
	if r.f(1) != 0 {
		return r.su(7)
	} else {
		return 0
	}
}

const MAX_TILE_WIDTH = 4096
const MAX_TILE_AREA = 4096 * 2304
const MAX_TILE_COLS = 64
const MAX_TILE_ROWS = 64

func tileInfo(r *Reader) int {
	var sbCols int
	var sbRows int
	var sbShift int

	if sh.use128x128Superblock {
		sbCols = (MiCols + 31) >> 5
		sbRows = (MiRows + 31) >> 5
		sbShift = 5
	} else {
		sbCols = (MiCols + 15) >> 4
		sbRows = (MiRows + 15) >> 4
		sbShift = 4
	}

	sbSize := sbShift + 2

	maxTileWidthSb := MAX_TILE_WIDTH >> sbSize
	maxTileAreaSb := MAX_TILE_AREA >> (2 * sbSize)

	minLog2TileCols := tileLog2(maxTileWidthSb, sbCols)
	maxLog2TileCols := tileLog2(1, min(sbCols, MAX_TILE_COLS))
	maxLog2TileRows := tileLog2(1, min(sbRows, MAX_TILE_ROWS))
	minLog2Tiles := max(minLog2TileCols, tileLog2(maxTileAreaSb, sbRows*sbCols))

	uniformTileSpacingFlag := r.f(1) != 0
	if uniformTileSpacingFlag {
		TileColsLog2 = minLog2TileCols
		for TileColsLog2 < maxLog2TileCols {
			if r.f(1) == 1 {
				TileColsLog2++
			} else {
				break
			}
		}

		tileWidthSb := (sbCols + (1 << TileColsLog2) - 1) >> TileColsLog2

		i := 0
		MiColStarts = make([]int, sbCols+1)
		for startSb := 0; startSb < sbCols; startSb += tileWidthSb {
			MiColStarts[i] = startSb << sbShift
			i += 1
		}
		MiColStarts[i] = MiCols
		TileCols = i

		minLog2TileRows := max(minLog2Tiles-TileColsLog2, 0)
		TileRowsLog2 = minLog2TileRows

		for TileRowsLog2 < maxLog2TileRows {
			if r.f(1) == 1 {
				TileRowsLog2++
			} else {
				break
			}
		}

		tileHeightSb := (sbRows + (1 << TileRowsLog2) - 1) >> TileRowsLog2
		i = 0
		MiRowStarts = make([]int, sbRows+1)
		for startSb := 0; startSb < sbRows; startSb += tileHeightSb {
			MiRowStarts[i] = startSb << sbShift
			i += 1
		}

		MiRowStarts[i] = MiRows
		TileRows = i
	} else {
		panic("no uniformTileSpacingFlag")
	}

	if TileColsLog2 > 0 || TileRowsLog2 > 0 {
		contextUpdateTileId := r.f(TileRowsLog2 + TileColsLog2)
		tileSizeBytesMinusOne := r.f(2)
		TileSizeBytes = tileSizeBytesMinusOne + 1
		return contextUpdateTileId
	} else {
		return 0
	}
}

func tileLog2(blkSize int, target int) int {
	var k int
	for k = 0; (blkSize << k) < target; k++ {
	}

	return k
}

func markRefRames(idLen int) {
	diffLen := sh.deltaFrameIdLengthMinusTwo + 2

	for i := 0; i < NUM_REF_FRAMES; i++ {
		if currentFrameId > (1 << diffLen) {
			if RefFrameId[i] > currentFrameId || RefFrameId[i] < (currentFrameId-(1<<diffLen)) {
				RefValid[i] = 0
			}
		} else {
			if RefFrameId[i] > currentFrameId && RefFrameId[i] < ((1<<idLen)+currentFrameId-(1<<diffLen)) {
				RefValid[i] = 0
			}
		}
	}
}

func frameSize(r *Reader, frameSizeOverrideFlag bool) {
	if frameSizeOverrideFlag {
		frameWidthMinusOne := r.f(sh.frameWidthBitsMinusOne + 1)
		frameHeightMinusOne := r.f(sh.frameHeightBitsMinusOne + 1)
		FrameWidth = frameWidthMinusOne + 1
		FrameHeight = frameHeightMinusOne + 1
	} else {
		FrameWidth = sh.maxFrameWidthMinusOne + 1
		FrameHeight = sh.maxFrameHeightMinusOne + 1
	}

	superresParams(r)
	computeImageSize()
}

const SUPERRES_DENOM_BITS = 3
const SUPERRES_DENOM_MIN = 9
const SUPERRES_NUM = 8

func superresParams(r *Reader) {
	useSuperres := false
	if sh.enableSuperres {
		useSuperres = r.f(1) != 0
	}

	if useSuperres {
		SuperresDenom = r.f(SUPERRES_DENOM_BITS) + SUPERRES_DENOM_MIN
	} else {
		SuperresDenom = SUPERRES_NUM
	}

	UpscaledWidth = FrameWidth
	FrameWidth = (UpscaledWidth*SUPERRES_NUM + (SuperresDenom / 2)) / SuperresDenom
}

func computeImageSize() {
	MiCols = 2 * ((FrameWidth + 7) >> 3)
	MiRows = 2 * ((FrameHeight + 7) >> 3)
}

func renderSize(r *Reader) {
	if r.f(1) != 0 {
		RenderWidth = r.f(16) + 1
		RenderHeight = r.f(16) + 1
	} else {
		RenderWidth = UpscaledWidth
		RenderHeight = FrameHeight
	}
}

const WARPEDMODEL_PREC_BITS = 16

func setupPastIndependence() {
	for i := 0; i < MAX_SEGMENTS; i++ {
		for j := 0; j < SEG_LVL_MAX; j++ {
			FeatureData[i][j] = 0
		}
	}

	PrevSegmentIds = make([][]int, MiRows)

	for row := 0; row < MiRows; row++ {
		PrevSegmentIds[row] = make([]int, MiCols)
	}

	PrevGmParams = make([][]int, ALTREF_FRAME+1)
	for ref := LAST_FRAME; ref <= ALTREF_FRAME; ref++ {
		PrevGmParams[ref] = make([]int, 6)
		for i := 0; i <= 5; i++ {
			if ref%3 == 2 {
				PrevGmParams[ref][i] = 1 << WARPEDMODEL_PREC_BITS
			} else {
				PrevGmParams[ref][i] = 0
			}
		}

	}
}
