package jpeg

type Section struct {
	Type byte
	Body []byte
}

func NewSection(inboundType byte, body []byte) *Section {
	return &Section{Type: inboundType, Body: body}
}

const (
	// parser will handle the 0xff prefix

	MARKER_DHT  byte = 0xc4
	MARKER_DQT  byte = 0xdb
	MARKER_DRI  byte = 0xdd
	MARKER_EOI  byte = 0xd9
	MARKER_SOF0 byte = 0xc0
	MARKER_SOI  byte = 0xd8
	MARKER_SOS  byte = 0xda

	// Non image
	MARKER_EXIF                   byte = 0xe1
	MARKER_JFIF                   byte = 0xe0
	MARKER_UNKNOWN_EXTENSION_MASK byte = 0xe0 // if it's not one of the named ones, i.e. JFIF or EXIF, we just mask it out
	MARKER_UNKNOWN_EXTENSION      byte = 0xe2 // We need to store it under something so I used 0xe2 to represent the rest of them

	// phony since no marker to start this. Just at the end of the scan
	MARKER_FRAME byte = 0x00
)
