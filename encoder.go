package main

// AMCITY encoder v1.
// Grid: v4 cell = /24 (256 addrs), v6 cell = /48. Cell value = majority tuple
// by covered address share inside the cell. Sections store runs of equal
// tuple over consecutive cells: (cellDelta uvarint, runLen uvarint, tid uvarint).
// Tuple ids are reassigned by total winning-cell weight (desc) so hot tuples
// get short varints. A sparse index every 1024 runs stores (absCell, offset).

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"os"
	"sort"
)

type Tup struct {
	Country string `json:"country"`
	Region  string `json:"region"`
	City    string `json:"city"`
}

type run struct {
	cell uint64
	len  uint64
	tid  uint32
}

func main() {
	dictRaw, err := os.ReadFile("dict.json")
	if err != nil {
		panic(err)
	}
	var rawDict [][3]string
	if err := json.Unmarshal(dictRaw, &rawDict); err != nil {
		panic(err)
	}
	dict := make([]Tup, len(rawDict))
	for i, t := range rawDict {
		dict[i] = Tup{t[0], t[1], t[2]}
	}
	fmt.Println("tuples:", len(dict))

	f, err := os.Open("ranges.bin")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	br := bufio.NewReaderSize(f, 1<<20)
	rec := make([]byte, 37)

	weight := map[uint32]uint64{} // old tid -> winning cells (for remap)

	// ---- v4 /22 majority via top-2 arrays ----
	const n22 = 1 << 24
	aTid := make([]uint32, n22)
	aW := make([]uint32, n22)
	bTid := make([]uint32, n22)
	bW := make([]uint32, n22)
	addV4 := func(cell uint32, tid uint32, w uint32) {
		if aW[cell] == 0 || aTid[cell] == tid {
			aTid[cell] = tid
			aW[cell] += w
		} else if bW[cell] == 0 || bTid[cell] == tid {
			bTid[cell] = tid
			bW[cell] += w
		} else if w > bW[cell] {
			bTid[cell] = tid
			bW[cell] = w
		}
	}

	// ---- v6: middle segments + partial-cell contributions ----
	type seg struct {
		cell uint64
		len  uint64
		tid  uint32
	}
	var v6segs []seg
	type contrib struct {
		tid uint32
		w   uint64
	}
	partial := map[uint64][]contrib{}
	addPartial := func(cell uint64, tid uint32, w uint64) {
		partial[cell] = append(partial[cell], contrib{tid, w})
	}

	one := big.NewInt(1)
	for {
		if _, err := io.ReadFull(br, rec); err != nil {
			break
		}
		fam := rec[0]
		tid := binary.BigEndian.Uint32(rec[33:37])
		if fam == 4 {
			s := binary.BigEndian.Uint32(rec[13:17])
			e := binary.BigEndian.Uint32(rec[29:33])
			first := s >> 8
			last := e >> 8
			for c := first; c <= last; c++ {
				cs := c << 8
				ce := cs + 255
				lo := s
				if cs > lo {
					lo = cs
				}
				hi := e
				if ce < hi {
					hi = ce
				}
				addV4(c, tid, hi-lo+1)
			}
		} else {
			s := new(big.Int).SetBytes(rec[1:17])
			e := new(big.Int).SetBytes(rec[17:33])
			firstCell := new(big.Int).Rsh(s, 80).Uint64()
			endCell := new(big.Int).Rsh(e, 80).Uint64()
			if endCell == firstCell {
				w := new(big.Int).Sub(e, s)
				w.Add(w, one)
				addPartial(firstCell, tid, w.Uint64())
			} else {
				cellEnd := new(big.Int).Lsh(new(big.Int).SetUint64(firstCell+1), 80)
				w1 := new(big.Int).Sub(cellEnd, s).Uint64()
				addPartial(firstCell, tid, w1)
				if endCell > firstCell+1 {
					v6segs = append(v6segs, seg{firstCell + 1, endCell - firstCell - 1, tid})
				}
				lastStart := new(big.Int).Lsh(new(big.Int).SetUint64(endCell), 80)
				w2 := new(big.Int).Sub(e, lastStart).Uint64() + 1
				addPartial(endCell, tid, w2)
			}
		}
	}
	fmt.Printf("v6 middle segments: %d, partial cells: %d\n", len(v6segs), len(partial))

	// ---- resolve v4 cells into segments (cell, len=1, tid) ----
	var v4segs []seg
	for c := 0; c < n22; c++ {
		if aW[c] == 0 {
			continue
		}
		win := aTid[c]
		if bW[c] > aW[c] {
			win = bTid[c]
		}
		v4segs = append(v4segs, seg{uint64(c), 1, win})
		weight[win]++
	}
	fmt.Println("v4 covered /24 cells:", len(v4segs))

	// ---- resolve v6 ----
	var v6all []seg
	v6all = append(v6all, v6segs...)
	for _, sg := range v6segs {
		weight[sg.tid] += sg.len
	}
	for cell, cs := range partial {
		agg := map[uint32]uint64{}
		var bestT uint32
		var bestW uint64
		for _, c := range cs {
			agg[c.tid] += c.w
			if agg[c.tid] > bestW {
				bestW = agg[c.tid]
				bestT = c.tid
			}
		}
		v6all = append(v6all, seg{cell, 1, bestT})
		weight[bestT]++
	}
	sort.Slice(v6all, func(i, j int) bool { return v6all[i].cell < v6all[j].cell })
	fmt.Println("v6 segments total:", len(v6all))

	// ---- remap tids by weight desc ----
	type wk struct {
		old uint32
		w   uint64
	}
	var wks []wk
	for old, w := range weight {
		wks = append(wks, wk{old, w})
	}
	sort.Slice(wks, func(i, j int) bool { return wks[i].w > wks[j].w })
	remap := make([]uint32, len(dict))
	newDict := make([]Tup, len(wks))
	for newID, e := range wks {
		remap[e.old] = uint32(newID)
		newDict[newID] = dict[e.old]
	}
	fmt.Println("tuples actually used:", len(wks))

	buildRuns := func(segs []seg) []run {
		var runs []run
		for _, sg := range segs {
			t := remap[sg.tid]
			if n := len(runs); n > 0 && runs[n-1].tid == t && runs[n-1].cell+runs[n-1].len == sg.cell {
				runs[n-1].len += sg.len
			} else {
				runs = append(runs, run{sg.cell, sg.len, t})
			}
		}
		return runs
	}
	v4runs := buildRuns(v4segs)
	v6runs := buildRuns(v6all)
	fmt.Println("v4 runs:", len(v4runs), " v6 runs:", len(v6runs))

	// ---- write file ----
	out, err := os.Create("amcity.bin")
	if err != nil {
		panic(err)
	}
	defer out.Close()
	bw := bufio.NewWriterSize(out, 1<<20)
	var tmp [binary.MaxVarintLen64]byte
	uv := func(v uint64) {
		n := binary.PutUvarint(tmp[:], v)
		bw.Write(tmp[:n])
	}
	bw.WriteString("AMCITY01")
	uv(uint64(len(newDict)))
	uv(uint64(len(v4runs)))
	uv(uint64(len(v6runs)))
	for _, t := range newDict {
		var cb [2]byte
		copy(cb[:], t.Country)
		bw.Write(cb[:])
		uv(uint64(len(t.Region)))
		bw.WriteString(t.Region)
		uv(uint64(len(t.City)))
		bw.WriteString(t.City)
	}
	writeSection := func(runs []run) int {
		var sec []byte
		var prev uint64
		type idxEnt struct{ cell, off uint64 }
		var idx []idxEnt
		var off uint64
		for i, r := range runs {
			if i%1024 == 0 {
				idx = append(idx, idxEnt{r.cell, off})
			}
			n := binary.PutUvarint(tmp[:], r.cell-prev)
			sec = append(sec, tmp[:n]...)
			off += uint64(n)
			n = binary.PutUvarint(tmp[:], r.len)
			sec = append(sec, tmp[:n]...)
			off += uint64(n)
			n = binary.PutUvarint(tmp[:], uint64(r.tid))
			sec = append(sec, tmp[:n]...)
			off += uint64(n)
			prev = r.cell + r.len
		}
		n := binary.PutUvarint(tmp[:], uint64(len(idx)))
		sec = append(sec, tmp[:n]...)
		for _, e := range idx {
			n = binary.PutUvarint(tmp[:], e.cell)
			sec = append(sec, tmp[:n]...)
			n = binary.PutUvarint(tmp[:], e.off)
			sec = append(sec, tmp[:n]...)
		}
		bw.Write(sec)
		return len(sec)
	}
	s4 := writeSection(v4runs)
	s6 := writeSection(v6runs)
	bw.Flush()
	st, _ := os.Stat("amcity.bin")
	fmt.Printf("section sizes: v4=%d v6=%d total file=%d bytes (%.1f MB)\n", s4, s6, st.Size(), float64(st.Size())/1048576)
}
