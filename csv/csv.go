package csv

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io/ioutil"
	"strconv"
)

// Filename ...
const Filename = "04-final-trace-2013.csv"

// ...
const (
	IDCount  = 63
	RowCount = 35036 // per ID
	ColCount = 6
)

// ...
const (
	DataID = iota
	Gen
	Grid
	Use
	Low
	Hi
)

// IDs ...
var IDs = []int{
	171, 1103, 1283, 1718, 1792, 370, 2072, 2233, 2337, 2470, 2755,
	2818, 2925, 2945, 2980, 2986, 3224, 3456, 3527, 3544, 3635, 3719,
	3723, 3918, 3935, 4193, 4302, 4357, 4447, 4526, 5129, 545, 585,
	744, 861, 890, 5246, 5275, 5357, 5439, 5615, 5738, 5785, 5796,
	5817, 5892, 5972, 6266, 6423, 6643, 6990, 7108, 7731, 7739, 7767,
	7863, 7989, 8084, 8155, 8626, 8829, 9121, 9631,
}

// Load ...
func Load(fname string) (map[int][][]float64, error) {
	b, err := ioutil.ReadFile(Filename)
	if err != nil {
		return nil, err
	}

	br := bytes.NewReader(b)
	cr := csv.NewReader(br)

	// Skip the headers
	if _, err := cr.Read(); err != nil {
		return nil, err
	}

	recs, err := cr.ReadAll()
	if err != nil {
		return nil, err
	}

	m := make(map[string][][]string)

	for i := 0; i < IDCount; i++ {
		m[recs[i*RowCount][DataID]] = recs[i*RowCount : (i+1)*RowCount]
	}

	return Convert(m)
}

// Convert ...
func Convert(m1 map[string][][]string) (map[int][][]float64, error) {
	m2 := make(map[int][][]float64)

	for k1, v1 := range m1 {
		k2, err := strconv.Atoi(k1)
		if err != nil {
			return nil, err
		}

		v2 := make([][]float64, len(v1))
		for row := 0; row < len(v1); row++ {
			auxFlt := make([]float64, ColCount-1)
			auxStr := make([]string, ColCount-1)
			v2[row] = make([]float64, ColCount-1) // drop the DataID column from v1

			var err error
			for col := 0; col < ColCount-1; col++ {
				auxFlt[col], err = strconv.ParseFloat(v1[row][col+1], 64)
				if err != nil {
					return nil, err
				}
				auxStr[col] = fmt.Sprintf("%.3f", auxFlt[col])
				v2[row][col], err = strconv.ParseFloat(auxStr[col], 64)
				if err != nil {
					return nil, err
				}
			}
		}

		m2[k2] = v2
	}

	return m2, nil
}
