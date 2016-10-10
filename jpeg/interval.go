package jpeg

import (
	"errors"
	"fmt"
	//"github.com/davecgh/go-spew/spew"
)

type Interval struct {
	Body []byte
	// MCU offset
	MCUOffset   int
	MCUs        int
	byteOffset  int
	bitCount    int
	workingByte byte
}

func NewInterval(b []byte, o int, m int) *Interval {
	return &Interval{Body: b, MCUOffset: o, MCUs: m, byteOffset: -1}
}

// Figure F.18

func (i *Interval) NextBit() (byte, error) {
	for i.bitCount == 0 {
		if i.byteOffset < (len(i.Body) - 1) {
			i.byteOffset += 1
			i.bitCount = 8
			i.workingByte = i.Body[i.byteOffset]
			//spew.Dump(s.workingByte)

			if i.workingByte == 0xFF {
				// peek ahead
				if i.byteOffset < len(i.Body)-1 {
					if i.Body[i.byteOffset+1] == 0x00 {
						//skip over pad byte. Note that we still process the existing byte
						//fmt.Println("We skipped over a 0x00 pad byte")
						i.byteOffset += 1
					} else {
						panic("malformed image data")
					}

				} else {
					panic("can't end on an 0xff")
				}

			}
		} else {
			return 0, errors.New("no data left")
		}
	}

	bit := i.workingByte >> 7
	i.bitCount -= 1
	i.workingByte <<= 1

	//fmt.Printf("end of nextbit and returning %d\n", bit)

	return bit, nil
}

func (i *Interval) NextBits(numBits int) int {
	//fmt.Printf("NextBits: %d requested and current byte offset (before reading) is %d\n", numBits, s.byteOffset)
	//fmt.Printf("In NextBits with arg of %d\n", numBits)

	var ret int = 0
	for j := 0; j < numBits; j++ {
		// Ignore error. Nextbit panics and we can leave this out for now since decoder should never
		// be reading out of bounds
		nextBit, _ := i.NextBit()
		ret <<= 1
		ret |= int(nextBit)
	}

	return ret

}
