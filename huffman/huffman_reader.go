package huffman

import (
	//"fmt"
	//"github.com/davecgh/go-spew/spew"
	"math"
)

const (
	TARGET_DC int = 0
	TARGET_AC int = 1
)

type NextBitProvider interface {
	NextBit() (byte, error)
	NextBits(numBits int) int
	PrintDebug()
}

type HuffmanReader struct {
	Target     int
	Identifier int
	HuffSize   map[int]int
	HuffCode   map[int]int
	HuffVal    []int
	MinCode    map[int]int
	MaxCode    map[int]int
	ValPtr     map[int]int
	Bits       map[int]int
}

// I think move NextBitProvider into initializer since it's integral to this class
func NewHuffmanReader(target int, identifier int, bits map[int]int, huffVal []int) *HuffmanReader {
	huff := &HuffmanReader{Bits: bits,
		HuffVal:    huffVal,
		Identifier: identifier,
		MaxCode:    make(map[int]int, 0),
		MinCode:    make(map[int]int, 0),
		Target:     target,
		ValPtr:     make(map[int]int, 0),
	}

	huff.generateSizeTable()
	huff.generateHuffCode()
	huff.generateDecodeTables()
	return huff
}

// From figure C.1
func (h *HuffmanReader) generateSizeTable() {
	huffSize := make(map[int]int)

	k := 0
	i := 1
	j := 1

	// This was another huge error. Getting "smart" with the flow diagrams
	for {
		if j > h.Bits[i] {
			i += 1
			j = 1
		} else {
			huffSize[k] = i
			k += 1
			j += 1
		}
		if i > 16 {
			huffSize[k] = 0
			break
		}
	}

	h.HuffSize = huffSize

}

// From figure C.2
func (h *HuffmanReader) generateHuffCode() {
	k := 0
	code := 0
	si := h.HuffSize[0]

	huffCode := make(map[int]int)

	for h.HuffSize[k] != 0 {
		huffCode[k] = code
		code += 1
		k += 1

		if h.HuffSize[k] == si {

			continue

		} else {

			if h.HuffSize[k] == 0 {

				continue // we'll break out at the top

			} else {

				// first time it's definitely not equal since that's the only reason we're in this branch.
				for h.HuffSize[k] != si {

					code <<= 1
					si += 1

				}

			}

		}
	}

	h.HuffCode = huffCode

}

// From figure F.15
func (h *HuffmanReader) generateDecodeTables() {
	i := 0
	j := 0

	for {
		i += 1
		if i > 16 {
			break
		} else {
			if h.Bits[i] == 0 {
				h.MaxCode[i] = -1
			} else {
				h.ValPtr[i] = j
				h.MinCode[i] = h.HuffCode[j]
				j += h.Bits[i] - 1
				h.MaxCode[i] = h.HuffCode[j]
				j += 1

			}

		}

	}

}

// Figure F.16

func (h *HuffmanReader) Decode(provider NextBitProvider) int {

	i := 1

	nb, err := provider.NextBit()

	code := int(nb)
	//fmt.Printf("i: %d, code: %v, maxCode[i]: %d\n", i, code, h.MaxCode[i])

	if err != nil {
		panic("shouldn't be trying to decode beyond end of frame")
	}

	for code > h.MaxCode[i] {
		i += 1
		code <<= 1
		nextBit, err := provider.NextBit()
		if err != nil {
			panic("shouldn't be trying to decode beyond end of frame")
		}
		code += int(nextBit)
	}

	j := h.ValPtr[i]

	j += code - h.MinCode[i]

	ret := h.HuffVal[j]

	return ret

}

func (h *HuffmanReader) ExtendVal(v int, t int) int {
	vt := int(math.Pow(2, float64(t-1)))

	if v < vt {
		vt = (-1 << uint(t)) + 1
		v += vt
	}

	return v

}

func (h *HuffmanReader) DecodeZZ(provider NextBitProvider, ssss int) int {

	//fmt.Printf("In DecodeZZ and ssss is %d\n", ssss)
	ret := int(provider.NextBits(ssss))
	ret = h.ExtendVal(ret, ssss)

	return ret
}

// Figure F.13
func (h *HuffmanReader) DecodeACCoefficients(provider NextBitProvider) [64]int {

	k := 1
	// The k = 0 position is the DC and will be handled outside this
	zz := [64]int{}

	for {
		//fmt.Printf("AC k: %d\n", k)
		rs := h.Decode(provider)

		ssss := rs % 16
		rrrr := rs >> 4

		r := rrrr

		if ssss == 0 {
			if r == 15 {
				k += 16
			} else {
				//fmt.Println("R is 15 so breaking")
				break
			}
		} else {
			k += r

			zz[k] = h.DecodeZZ(provider, ssss)

			if k == 63 {
				break
			} else {
				k += 1
			}

		}

	}

	return zz

}

// Later handle previous dc. Unclear if it's pre or post centering
func (h *HuffmanReader) DecodeDC(provider NextBitProvider, prev int) int {
	val := h.Decode(provider)

	next7Bits := provider.NextBits(val)

	extendedVal := h.ExtendVal(int(next7Bits), val)

	return extendedVal + prev

}

func (h *HuffmanReader) DeZigZag(zigZag [64]int) [64]int {
	out := [64]int{}

	r := 0
	c := 1
	direction := -1

	cWantsMove := 0
	rWantsMove := 0

	// yuck but fun to reason about. Talked to colleague and he said just do this with lookup table to be faster

	for i := 0; i < 63; i++ {
		out[r*8+c] = zigZag[i+1]
		if direction == -1 {

			if c > 0 {
				cWantsMove = -1
			} else {
				cWantsMove = 0
			}

			if r < 7 {
				rWantsMove = 1
			} else {
				rWantsMove = 0
			}

		} else {
			if c < 7 {
				cWantsMove = 1
			} else {
				cWantsMove = 0
			}

			if r > 0 {
				rWantsMove = -1
			} else {
				rWantsMove = 0
			}

		}

		// Both can move so move
		if cWantsMove != 0 && rWantsMove != 0 {
			c += cWantsMove
			r += rWantsMove
		}

		if cWantsMove == 0 && rWantsMove != 0 {
			// After half, this needs to switch
			if i > 32 {
				rWantsMove *= -1

			}
			r += rWantsMove
			direction *= -1
		}

		if cWantsMove != 0 && rWantsMove == 0 {
			// After half, this needs to switch
			if i > 32 {
				cWantsMove *= -1
			}
			c += cWantsMove
			direction *= -1
		}

		if cWantsMove == 0 && rWantsMove == 0 {
			c += 1 // bottom left
			direction *= -1
		}

	}

	return out

}
