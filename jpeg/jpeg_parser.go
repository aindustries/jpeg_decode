package jpeg

import (
	"bytes"
	"fmt"
	"huffman"
	"io/ioutil"
)

type JpegParser struct {
	XLines             int
	YLines             int
	QuantizationTables map[int][64]int
	ByteReader         *bytes.Reader
	Sections           map[byte]*Section
	HuffmanReaders     []*huffman.HuffmanReader
	RestartInterval    int
	Intervals          []*Interval
}

func NewJpegParser(filename string) *JpegParser {
	j := &JpegParser{}
	j.Sections = make(map[byte]*Section)
	j.QuantizationTables = make(map[int][64]int)
	j.HuffmanReaders = make([]*huffman.HuffmanReader, 0)

	rawBytes, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	j.ByteReader = bytes.NewReader(rawBytes)

	j.ParseSections()
	j.ReadHuffmanTables()
	j.ReadQuantizationTables()
	j.ParseStartOfFrame()
	j.ParseRestart()

	return j
}

func (j *JpegParser) MCUCols() int {

	ret := j.XLines / 16

	if j.XLines%16 > 0 {
		ret++
	}

	return ret
}

func (j *JpegParser) MCURows() int {

	ret := j.YLines / 16

	if j.YLines%16 > 0 {
		ret++
	}

	return ret
}

// This method is crying for a rewrite. Also the look ahead reader on marker byte is poorly done. Should be only single
// read call
func (j *JpegParser) ParseSections() {
	var readError error
	var b byte

	frameStart := 0
	inFrame := false

	offset := 0

	offsetOfEOI := 0

	readError = nil

	//Named break. Redo
reader:
	for {
		b, readError = j.ByteReader.ReadByte()
		if readError != nil {
			break
		}
		offset += 1

		if b == 0xFF {

			if j.ByteReader.Len() == 0 {
				fmt.Printf("no more bytes in buffer\n")
				// EOF marker so continue
				break
			}

			nextByte, readError := j.ByteReader.ReadByte()
			offset += 1

			// This is dirty - the error check vs eof check
			if readError != nil {
				panic(readError)
			}

			var markerType byte

			if inFrame && nextByte != MARKER_EOI {
				continue // the frame has numerous things that look like markers that aren't
			}

			switch {
			// In one of the cases we have to mask nextByte instead of just checking equality
			case nextByte == MARKER_SOI:
				markerType = MARKER_SOI
				// SOI is only a marker. It doesn't have a section that follows
				continue
			case nextByte == MARKER_DHT:
				markerType = MARKER_DHT
			case nextByte == MARKER_DRI:
				markerType = MARKER_DRI
			case nextByte == MARKER_DQT:
				markerType = MARKER_DQT
			case nextByte == MARKER_EXIF:
				markerType = MARKER_EXIF
			case nextByte == MARKER_JFIF:
				//fmt.Printf("Marker is jfif and byte is %d\n", nextByte)
				markerType = MARKER_JFIF
			case nextByte == MARKER_SOF0:
				markerType = MARKER_SOF0
			case nextByte == MARKER_SOS:
				markerType = MARKER_SOS
			case nextByte&MARKER_UNKNOWN_EXTENSION_MASK == MARKER_UNKNOWN_EXTENSION_MASK:
				//fmt.Printf("Marker is unknown extension and byte is %d\n", nextByte)
				markerType = MARKER_UNKNOWN_EXTENSION
			case nextByte == MARKER_EOI:
				// only applicable inside frame
				if !inFrame {
					panic("EOI found and we're not in frame")
				}
				markerType = MARKER_EOI
				//fmt.Printf("setting lastEOI to %d\n", offset)
				offsetOfEOI = offset - 2

				break reader // and we need to skip over the rest of the loop here to prevent a read beyond the EOI.
				// Some writers (Adobe photoshop being the one in the tests)  put info beyond the EOI that we must ignore

			default:
				panic("unknown marker hit. Exiting")
			}

			//fmt.Printf("We think offset at end of marker: %v is: %d\n", markerType, offset)

			lenSectionAsBytes := make([]byte, 2)

			r, err := j.ByteReader.Read(lenSectionAsBytes)

			if r != 2 || err != nil {
				panic("malformed on marker read")
			}

			offset += 2

			sectionLength := (int(lenSectionAsBytes[0]) << 8) | int(lenSectionAsBytes[1])

			//fmt.Printf("for markerType: %d we read numBytes as: %d\n", markerType, sectionLength)

			//fmt.Printf("found a %v with length %v\n", markerType, sectionLength)

			sectionLength -= 2 // As stored, includes length bytes

			offset += sectionLength

			sectionBody := make([]byte, sectionLength)

			r, err = j.ByteReader.Read(sectionBody)

			if r != sectionLength || err != nil {
				panic(r)
				//panic("malformed on section body read")
			}

			// Some jpeg writers duplicate sections instead of having a single. So we check if the section already exists
			// and if so we append the bytes. Otherwise we create a new section

			existingSection, present := j.Sections[markerType]

			if present {
				existingSection.Body = append(existingSection.Body, sectionBody...) // The ... syntax is wacky
			} else {
				section := NewSection(markerType, sectionBody)
				j.Sections[markerType] = section
			}

			// Dirty but seemingly no marker on frame start

			if markerType == MARKER_SOS {
				if present {
					// marker SOS should never be duped so panic if it does as a precaution
					panic("Marker SOS duped")
				}
				// we will read from the end of it to get the frame
				//fmt.Printf("We think frameStart is %d\n", offset)
				frameStart = offset
				inFrame = true
			}

		}
	}

	if _, p := j.Sections[MARKER_SOS]; !p {
		panic("got to the end of parsing without a scan start")
	}
	// We are done but we need to read behind us to get the frame

	//fmt.Printf("offsetOfEOI: %d, frameStart: %d\n", offsetOfEOI, frameStart)
	frameLength := offsetOfEOI - frameStart

	frameBody := make([]byte, frameLength)

	r, err := j.ByteReader.ReadAt(frameBody, int64(frameStart))

	if r != len(frameBody) || err != nil {
		panic(r)
	}

	frameMarkerType := MARKER_FRAME

	section := NewSection(frameMarkerType, frameBody)

	j.Sections[frameMarkerType] = section
}

func (j *JpegParser) ParseRestart() {
	sec, present := j.Sections[MARKER_DRI]

	frame := j.Sections[MARKER_FRAME]

	// We need to write a final interval regardless of whether this frame uses restarts. It could be
	// the final interval in a series or the only interval in the entire frame

	XMCUs := j.XLines / 16
	// Round up if it's not a multiple of 16
	if j.XLines%16 != 0 {
		XMCUs++
	}

	YMCUs := j.YLines / 16

	if j.YLines%16 != 0 {
		YMCUs++
	}

	totalMCUsExpected := XMCUs * YMCUs

	// Below is done only if there are restart markers. Otherwise the remainder math below this
	// will make a single interval out of the "remainder" of the file (which may be a single, giant interval)

	markerCount := 0

	if present {
		j.RestartInterval = int(sec.Body[0]<<8) | int(sec.Body[1])
		j.Intervals = make([]*Interval, 0)

		frameLength := len(frame.Body)

		prevByteFF := false
		intervalStart := 0
		intervalEnd := 0

		for byteIndex := 0; byteIndex < frameLength; byteIndex++ {
			restartIndex := markerCount % 8

			b := frame.Body[byteIndex]

			if b == 0xFF {
				prevByteFF = true
				continue
			}

			if prevByteFF {
				prevByteFF = false
				if b == 0x00 {
					// This is a padding byte so continue
					continue
				}

				// Marker bytes are 0xffd0 - 0xffd7. 0xd0 in decimal is 208 so subtract by 208 so we can compare with 0 - 7

				if int(b)-208 == restartIndex {
					// Here's the interesting part:
					// byteIndex - 2 is the end of the last interval
					// byteIndex + 1 is the start of the next interval (but we need to copy out the bytes before we reset it)

					intervalEnd = byteIndex - 2
					// Go slice subsetting isn't inclusive so add 1
					interval := NewInterval(frame.Body[intervalStart:intervalEnd+1], markerCount*j.RestartInterval, j.RestartInterval)

					j.Intervals = append(j.Intervals, interval)

					intervalStart = byteIndex + 1
					markerCount++

				} else {
					fmt.Printf("byte found: %d\n", b)
					panic("Found a marker in the frame body that we don't expect")
				}

			}

		}

		remainder := totalMCUsExpected - markerCount*j.RestartInterval

		// Panic if the math is wrong since it's unrecoverable. Note this only applies to situations where there
		// is a restart interval defined. Otherwise the restart interval will be 0 and the entire frame is the
		// remainder

		if j.RestartInterval != 0 {

			if remainder > j.RestartInterval {
				panic("Math on MCUs in this image is wrong - unrecoverable")
			}
		}

		interval := NewInterval(frame.Body[intervalStart:len(frame.Body)], markerCount*j.RestartInterval, remainder)
		j.Intervals = append(j.Intervals, interval)

	} else {
		j.Intervals = []*Interval{NewInterval(frame.Body, 0, totalMCUsExpected)}
	}

}

func (j *JpegParser) ParseStartOfFrame() {

	sof := j.Sections[MARKER_SOF0]

	offset := 1 // skip precision byte

	yLines := int(sof.Body[offset])<<8 | int(sof.Body[offset+1])

	offset += 2

	xLines := int(sof.Body[offset])<<8 | int(sof.Body[offset+1])

	offset += 2
	//skip rest since they're standard

	j.XLines = xLines
	j.YLines = yLines

	//skip component data

	//numComponents := int(sof.Body[offset])

	//fmt.Println(numComponents)

}

func (j *JpegParser) ReadQuantizationTables() {

	dqt := j.Sections[MARKER_DQT]

	offset := 0

	//fmt.Printf("offset: %v len: %v\n", offset, len(dqt.Body))

	for offset < len(dqt.Body) {
		tableId := int(dqt.Body[offset] & 0x0F)

		offset += 1

		byteSlice := dqt.Body[offset : offset+64]

		intArray := [64]int{}

		for index, element := range byteSlice {
			intArray[index] = int(element)
		}

		j.QuantizationTables[tableId] = intArray
		offset += 64
		//fmt.Printf("offset: %v len: %v\n", offset, len(dqt.Body))
	}

}

func (j *JpegParser) ReadHuffmanTables() {

	dht := j.Sections[MARKER_DHT]

	offset := 0

	for offset < len(dht.Body) {

		target := int((dht.Body[offset] & 0xF0) >> 4)
		identifier := int(dht.Body[offset] & 0x0F)

		//fmt.Printf("target: %d identifier %d\n", target, identifier)

		offset += 1

		counter := 1

		bits := make(map[int]int)

		for ; counter <= 16; counter += 1 {
			bits[counter] = int(dht.Body[offset])
			offset += 1
		}

		// Next we need to sum bits to determine how many huffval we will have

		totalVals := 0
		for _, v := range bits {
			totalVals += v
		}

		huffVal := make([]int, totalVals)

		for i := 0; i < totalVals; i += 1 {
			huffVal[i] = int(dht.Body[offset])
			offset += 1
		}

		huffmanReader := huffman.NewHuffmanReader(target, identifier, bits, huffVal)

		//fmt.Printf("Adding a huffman reader with huffval length: %d\n", len(huffVal))
		// Handle Huffman reader init
		j.HuffmanReaders = append(j.HuffmanReaders, huffmanReader)
	}

}

func (j *JpegParser) GetHuffmanReader(target int, identifier int) *huffman.HuffmanReader {
	var ret *huffman.HuffmanReader

	for _, e := range j.HuffmanReaders {
		if e.Target == target && e.Identifier == identifier {
			ret = e
		}
	}

	return ret

}
