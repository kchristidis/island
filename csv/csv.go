package csv

import (
	"bytes"
	"encoding/csv"
	"io/ioutil"
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
func Load(fname string) (map[string][][]string, error) {
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

	return m, nil
}
