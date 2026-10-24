package plan

import (
	"reflect"
	"testing"
	"time"
)

func h(v uint64) *uint64 { return &v }
func n(v int) *int       { return &v }

func TestSimilarGroups(t *testing.T) {
	day := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	other := day.Add(time.Hour)
	photos := []Photo{
		{MediaID: 1, PHash: h(0xF0F0F0F0F0F0F0F0)},
		{MediaID: 2, PHash: h(0xF0F0F0F0F0F0F0F3)}, // 2 bits off 1 -> alike
		{MediaID: 3, PHash: h(0xF0F0F0F0F0F0F03F)}, // 6 bits off 2 -> alike (transitive group with 1)
		{MediaID: 4, PHash: h(0x0F0F0F0F0F0F0F0F)}, // far from all
		{MediaID: 5, Name: "IMG_1.jpg", DateTaken: &day, Width: n(4000), Height: n(3000)},
		{MediaID: 6, Name: "IMG_1.jpg", DateTaken: &day, Width: n(4000), Height: n(3000)},   // same name+date+size
		{MediaID: 7, Name: "IMG_1.jpg", DateTaken: &other, Width: n(4000), Height: n(3000)}, // other time: different photo
		{MediaID: 8, Name: "IMG_2.jpg", DateTaken: &day, Width: n(4000), Height: n(3000)},   // other name
		{MediaID: 9}, // nothing to compare
	}
	got := SimilarGroups(photos)
	want := []Group{
		{MediaIDs: []int64{1, 2, 3}, Reason: "looks_alike"},
		{MediaIDs: []int64{5, 6}, Reason: "same_name_and_date"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestSimilarThresholdBoundary(t *testing.T) {
	base := uint64(0)
	at := uint64(1<<SimilarMaxDistance - 1)       // exactly SimilarMaxDistance bits set
	over := uint64(1<<(SimilarMaxDistance+1) - 1) // one more
	if g := SimilarGroups([]Photo{{MediaID: 1, PHash: &base}, {MediaID: 2, PHash: &at}}); len(g) != 1 {
		t.Fatal("distance == threshold should group")
	}
	if g := SimilarGroups([]Photo{{MediaID: 1, PHash: &base}, {MediaID: 2, PHash: &over}}); len(g) != 0 {
		t.Fatal("distance > threshold should not group")
	}
	if g := SimilarGroups(nil); g != nil {
		t.Fatal("empty input")
	}
}
