package main

import (
	"flag"
	"fmt"
	// below for writing outputs
	"image"
	"image/color"
	golangPng "image/png"
	"os"

	// Mine - extracted from their own projects
	"dct"
	"huffman"
	"jpeg"
)

func fileDecodeRead(jpegReader *jpeg.JpegParser, interval *jpeg.Interval, stringIdentifier string, previousDC int) ([64]int, int) {

	identifier := 0 // luma
	if stringIdentifier == "chroma" {
		identifier = 1
	}

	array := [64]int{}

	dcReader := jpegReader.GetHuffmanReader(huffman.TARGET_DC, identifier)
	acReader := jpegReader.GetHuffmanReader(huffman.TARGET_AC, identifier)

	dcToReturn := dcReader.DecodeDC(interval, previousDC)

	array[0] = dcToReturn

	zigZag := acReader.DecodeACCoefficients(interval)

	for i := 1; i < 64; i++ {
		array[i] = zigZag[i]
	}

	// Now Dequantize and recenter

	table := jpegReader.QuantizationTables[identifier]

	for i := 0; i < 64; i++ {
		array[i] *= table[i]
	}

	// Now de-zig-zag

	straightened := acReader.DeZigZag(array)
	for i := 1; i < 64; i++ {
		array[i] = straightened[i]
	}

	dctTransformer := dct.NewTransformer()

	array = dctTransformer.ArrayToArrayIDCT(array)

	// Recenter and clamp

	for i, _ := range array {
		array[i] += 128
	}

	for i, _ := range array {
		array[i] = intClamp(array[i])
	}

	return array, dcToReturn

}

func intClamp(val int) int {
	if val > 255 {
		val = 255
	} else if val < 0 {
		val = 0
	}
	return val
}

func decodeInterval(j *jpeg.JpegParser, interval *jpeg.Interval, clrImg *image.RGBA) {
	lumas := [4][64]int{}

	cbArray := [64]int{}
	crArray := [64]int{}

	// Always zero at start of an interval
	lumaDC := 0
	cbDC := 0
	crDC := 0

	for i := 0; i < interval.MCUs; i++ {
		// Used below but also for debugging
		thisMCU := interval.MCUOffset + i

		lumas[0], lumaDC = fileDecodeRead(j, interval, "luma", lumaDC)
		lumas[1], lumaDC = fileDecodeRead(j, interval, "luma", lumaDC)
		lumas[2], lumaDC = fileDecodeRead(j, interval, "luma", lumaDC)
		lumas[3], lumaDC = fileDecodeRead(j, interval, "luma", lumaDC)

		cbArray, cbDC = fileDecodeRead(j, interval, "chroma", cbDC)
		crArray, crDC = fileDecodeRead(j, interval, "chroma", crDC)

		// Need to rebuild col and row here

		c := thisMCU % j.MCUCols()
		r := thisMCU / j.MCUCols()
		//fmt.Printf("sending in c: %d, r: %d for thisMCU: %d, intervalOffset: %d\n", c, r, thisMCU, interval.MCUOffset)

		yCbCrArraysToImage(lumas, cbArray, crArray, clrImg, c*16, r*16)
	}

}
func doFileDecode(desiredFile *string) image.Image {
	j := jpeg.NewJpegParser(*desiredFile)

	outImgX := j.XLines + (j.XLines % 16) // No op or plus 8
	outImgY := j.YLines + (j.YLines % 16) // No op or plus 8

	fmt.Printf("Input XLines: %d, YLines: %d\n", j.XLines, j.YLines)

	var colorImg *image.RGBA
	colorImg = image.NewRGBA(image.Rect(0, 0, outImgX, outImgY))
	colorImg.Stride = outImgX * 4 // 4 bytes per pixels (rgba8)

	fmt.Printf("Restart length is %d\n", j.RestartInterval)

	for _, interval := range j.Intervals {
		fmt.Println("---------------Starting new interval---------------")
		decodeInterval(j, interval, colorImg)
	}

	return colorImg
}

func yCbCrArraysToImage(lumas [4][64]int, cbArray [64]int, crArray [64]int, clrImg *image.RGBA, xOffset int, yOffset int) {

	for lumaI, lumaE := range lumas {
		for row := 0; row < 8; row++ {
			for col := 0; col < 8; col++ {

				lumaIndex := row*8 + col
				// This is important. It operates in a 1:2 in each dimension basis to take into chroma sub-sampling account
				chromaIndex := (row/2)*8 + col/2
				//fmt.Printf("chromaIndex: %d\n", chromaIndex)

				intraBlockXOffset := 0
				if lumaI == 1 || lumaI == 3 {
					intraBlockXOffset = 8
					chromaIndex += 4 // right side
				}

				intraBlockYOffset := 0
				if lumaI == 2 || lumaI == 3 {
					intraBlockYOffset = 8
					chromaIndex += 4 * 8 // bottom half
				}

				luma := float64(lumaE[lumaIndex])
				cb := float64(cbArray[chromaIndex])
				cr := float64(crArray[chromaIndex])

				// This was a problem since it's not in the itu 81 spec. link to JFIF spec
				// https://www.w3.org/Graphics/JPEG/jfif3.pdf
				r := luma + 1.402*(cr-128)
				g := luma - 0.34414*(cb-128) - 0.71414*(cr-128)
				b := luma + 1.772*(cb-128)

				clr := color.RGBA{uint8(intClamp(int(r))), uint8(intClamp(int(g))), uint8(intClamp(int(b))), 255}

				clrImg.Set(col+intraBlockXOffset+xOffset, row+intraBlockYOffset+yOffset, clr)
			}
		}
	}

}

func writeAsPngUsingGolangEncoder(inImg image.Image, name string) {
	f, _ := os.Create(name)

	if err := golangPng.Encode(f, inImg); err != nil {
		panic(err)
	}
}

func main() {
	inImgPtr := flag.String("image", "spec.jpg", "desired input file")

	flag.Parse()
	flag.Usage()
	if *inImgPtr != "" {
		img := doFileDecode(inImgPtr)
		writeAsPngUsingGolangEncoder(img, "/tmp/out.png") // For debugging
	}

}
