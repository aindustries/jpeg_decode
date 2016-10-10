package dct

import (
	//"fmt"
	"math"
)

type Transformer struct{}

func NewTransformer() *Transformer {
	return &Transformer{}
}

func (t *Transformer) Round(f float64) int {

	var ret int

	if f > 0 {
		ret = int(f + 0.5)
	} else {
		ret = int(f - 0.5)
	}

	return ret

}

func (t *Transformer) ArrayToArrayIDCT(in [64]int) [64]int {

	// first we need to make a matrix of float 64s

	matrix := [8][8]float64{}

	for i, e := range in {
		matrix[i/8][i%8] = float64(e)
	}

	outMatrix := [8][8]float64{}

	for x := 0; x < 8; x++ {

		for y := 0; y < 8; y++ {

			for u := 0; u < 8; u++ {
				for v := 0; v < 8; v++ {

					multiplier := 1.0

					if u == 0 {
						multiplier /= math.Pow(2, 0.5)
					}

					if v == 0 {
						multiplier /= math.Pow(2, 0.5)
					}

					outMatrix[y][x] += multiplier * matrix[v][u] * math.Cos(((2*float64(x)+1)*float64(u)*math.Pi)/16) * math.Cos(((2*float64(y)+1)*float64(v)*math.Pi)/16)

				}
			}
		}
	}

	out := [64]int{}

	for i, _ := range out {
		out[i] = t.Round((outMatrix[i/8][i%8]) * 0.25)
	}

	return out

}
