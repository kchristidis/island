package trace

import (
	"bytes"
	"encoding/csv"
	"io"
	"io/ioutil"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadTrace(t *testing.T) {
	b, err := ioutil.ReadFile(Filename)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("all at once", func(t *testing.T) {
		br := bytes.NewReader(b)
		cr := csv.NewReader(br)

		_, err = cr.ReadAll()
		if err != nil {
			t.Fatal(err)
		}

		_, err = cr.Read()
		if err != io.EOF {
			t.Fatal(err)
		}
	})

	t.Run("one line at a time", func(t *testing.T) {
		br := bytes.NewReader(b)
		cr := csv.NewReader(br)

		rec, err := cr.Read()
		if err != nil {
			t.Fatal(err)
		}

		if len(rec) != ColCount {
			t.Fatalf("wrong number of columns (%d)", len(rec))
		}
	})

	t.Run("encode into appropriate data structure", func(t *testing.T) {
		br := bytes.NewReader(b)
		cr := csv.NewReader(br)

		cr.Read() // skip the headers

		recs, _ := cr.ReadAll()

		firstFirst := recs[0]
		firstLast := recs[0+RowCount-1]
		secondFirst := recs[RowCount]

		m := make(map[string][][]string)

		for i := 0; i < IDCount; i++ {
			m[recs[i*RowCount][DataID]] = recs[i*RowCount : (i+1)*RowCount]
		}

		for j := 0; j < ColCount; j++ {
			if m[strconv.Itoa(IDs[0])][0][j] != firstFirst[j] {
				t.Fatalf("expected %s, got %s", firstFirst, m[strconv.Itoa(IDs[0])][0])
			}
			if m[strconv.Itoa(IDs[0])][len(m[strconv.Itoa(IDs[0])])-1][j] != firstLast[j] {
				t.Fatalf("expected %s, got %s", firstFirst, m[strconv.Itoa(IDs[0])][0])
			}
			if m[strconv.Itoa(IDs[1])][0][j] != secondFirst[j] {
				t.Fatalf("expected %s, got %s", firstFirst, m[strconv.Itoa(IDs[0])][0])
			}
		}

	})
}

func TestConversion(t *testing.T) {
	m1 := map[string][][]string{
		"171": [][]string{
			[]string{"171", "-0.005", "0.8782666666666666", "0.8782666666666666", "3.4", "10.84"},
		},
	}

	m2exp := map[int][][]float64{
		171: [][]float64{
			[]float64{-0.005, 0.878, 0.878, 3.4, 10.84},
		},
	}

	m2act, err := Convert(m1)
	require.NoError(t, err)
	require.Equal(t, 1, len(m2act[171]))
	require.Equal(t, ColCount-1, len(m2act[171][0]))
	require.InDeltaSlice(t, m2exp[171][0], m2act[171][0], 0.0001)
	t.Log(m1["171"][0])
	t.Log(m2act[171][0])
}
