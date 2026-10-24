package plan

import (
	"math/bits"
	"sort"
	"time"
)

// SimilarMaxDistance is the largest Hamming distance between two 64-bit perceptual hashes
// (dHash/pHash computed by the phone) that still counts as "looks the same". About 10% of bits.
const SimilarMaxDistance = 6

// Photo is the information used to decide whether two different files are the same picture.
type Photo struct {
	MediaID   int64
	PHash     *uint64
	Name      string
	DateTaken *time.Time
	Width     *int
	Height    *int
}

// Group is a set of look-alike photos (each a different file).
type Group struct {
	MediaIDs []int64
	Reason   string // looks_alike | same_name_and_date
}

// SimilarGroups clusters photos that look alike (close perceptual hashes) or share
// name + date taken + dimensions (e.g. a re-saved copy). Identical bytes never reach here: those are
// one media row. Groups are transitive (A~B, B~C puts A, B, C together) and sorted for stable output.
//
// Pairwise comparison is O(n²); fine for the proof of concept, to be replaced by bucketing (e.g.
// splitting hashes into bands) before libraries reach tens of thousands of photos.
func SimilarGroups(photos []Photo) []Group {
	parent := make([]int, len(photos))
	reason := make([]string, len(photos))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	union := func(a, b int, why string) {
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		parent[rb] = ra
		if reason[ra] == "" || why == "looks_alike" {
			reason[ra] = why
		}
	}
	for i := 0; i < len(photos); i++ {
		for j := i + 1; j < len(photos); j++ {
			a, b := photos[i], photos[j]
			switch {
			case a.PHash != nil && b.PHash != nil && bits.OnesCount64(*a.PHash^*b.PHash) <= SimilarMaxDistance:
				union(i, j, "looks_alike")
			case sameNameAndDate(a, b):
				union(i, j, "same_name_and_date")
			}
		}
	}
	byRoot := map[int][]int{}
	for i := range photos {
		r := find(i)
		byRoot[r] = append(byRoot[r], i)
	}
	var out []Group
	for root, members := range byRoot {
		if len(members) < 2 {
			continue
		}
		g := Group{Reason: reason[root]}
		for _, m := range members {
			g.MediaIDs = append(g.MediaIDs, photos[m].MediaID)
		}
		sort.Slice(g.MediaIDs, func(i, j int) bool { return g.MediaIDs[i] < g.MediaIDs[j] })
		out = append(out, g)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MediaIDs[0] < out[j].MediaIDs[0] })
	return out
}

func sameNameAndDate(a, b Photo) bool {
	return a.Name != "" && a.Name == b.Name &&
		a.DateTaken != nil && b.DateTaken != nil && a.DateTaken.Equal(*b.DateTaken) &&
		a.Width != nil && b.Width != nil && *a.Width == *b.Width &&
		a.Height != nil && b.Height != nil && *a.Height == *b.Height
}
