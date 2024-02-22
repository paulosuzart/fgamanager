package db

import "testing"

func TestCount(t *testing.T) {
	setupDb(":memory:")
	defer Close()

	if c := CountTuples(nil); c != 0 {
		t.Error("There must be no entries")
	}
}
