package db

import (
	openfga "github.com/openfga/go-sdk"
	"testing"
	"time"
)

func TestCount(t *testing.T) {
	setupDb(":memory:")
	defer Close()

	t.Run("Count ok on write", func(subtest *testing.T) {
		// given
		tupleChange := openfga.TupleChange{
			TupleKey: openfga.TupleKey{
				User:     "user:jack",
				Relation: "member",
				Object:   "group:boss"},
			Operation: openfga.WRITE,
			Timestamp: time.Now()}

		// when
		ApplyChange(tupleChange)

		// then
		if c := Repository.CountTuples(nil); c != 1 {
			t.Error("There must be 1 entry")
		}
	})

	t.Run("Count ok on deletion", func(t *testing.T) {
		// given
		tupleChange := openfga.TupleChange{
			TupleKey: openfga.TupleKey{
				User:     "user:jack",
				Relation: "member",
				Object:   "group:boss"},
			Operation: openfga.DELETE,
			Timestamp: time.Now()}

		// when
		ApplyChange(tupleChange)

		// then
		if c := Repository.CountTuples(nil); c != 0 {
			t.Error("There must be 1 entry")
		}
	})

}
