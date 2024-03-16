package main

import (
	"context"
	openfga "github.com/openfga/go-sdk"
	"github.com/paulosuzart/fgamanager/db"
	"net/http"
	"testing"
	"time"
)

type mockRepo struct {
	db.TupleRepository
	GetMarkedForDeletionFunc func() []db.Tuple
}

func (r mockRepo) GetMarkedForDeletion() []db.Tuple {
	return r.GetMarkedForDeletionFunc()
}

func (r mockRepo) CountTuples(_ *db.Filter) int {
	return 0
}

type mockFga struct {
	fgaService
	writeFunc  func(ctx context.Context, tuple *openfga.WriteRequestWrites) error
	deleteFunc func(ctx context.Context, deletes []openfga.TupleKeyWithoutCondition) (*http.Response, error)
}

func (m mockFga) delete(ctx context.Context, deletes []openfga.TupleKeyWithoutCondition) (*http.Response, error) {
	return m.deleteFunc(ctx, deletes)
}

func (m mockFga) write(ctx context.Context, tuple *openfga.WriteRequestWrites) error {
	return m.writeFunc(ctx, tuple)
}

func Test(t *testing.T) {

	db.Repository = mockRepo{
		GetMarkedForDeletionFunc: func() []db.Tuple {
			return []db.Tuple{
				{
					TupleKey:   "user:jack member org:acme",
					UserType:   "user",
					UserId:     "jack",
					Relation:   "member",
					ObjectType: "org",
					ObjectId:   "acme",
				},
			}
		},
	}
	t.Run("Test Delete", func(t *testing.T) {
		invokedChan := make(chan interface{})
		fga = mockFga{deleteFunc: func(ctx context.Context, deletes []openfga.TupleKeyWithoutCondition) (*http.Response, error) {
			if len(deletes) == 0 {
				t.Error("At least one tuple for deletion is expected")
			}
			if deletes[0].User != "user:jack" {
				t.Error("User data mismatch")
			}
			invokedChan <- true
			return &http.Response{StatusCode: 200}, nil
		}}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		go deleteMarked(ctx)
		<-invokedChan
		cancel()
	})

	t.Run("Test Write valid tuple string", func(t *testing.T) {
		var called = false
		fga = mockFga{writeFunc: func(ctx context.Context, tuple *openfga.WriteRequestWrites) error {
			if tuple == nil {
				t.Fatal("Tuple to be written can't be null. One tuple expected")
			}
			called = true
			return nil
		}}
		ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
		create(ctx, "folder:zoo owner doc:turtles")
		if !called {
			t.Error("No write not called")
		}

	})

	t.Run("Test Write invalid tuple string", func(t *testing.T) {
		var called = false
		fga = mockFga{writeFunc: func(ctx context.Context, tuple *openfga.WriteRequestWrites) error {
			if tuple == nil {
				t.Fatal("Tuple to be written can't be null. One tuple expected")
			}
			called = true
			return nil
		}}
		ctx, _ := context.WithTimeout(context.Background(), 3*time.Second)
		create(ctx, "folder:zoo owner h doc:turtles")
		if called {
			t.Error("Write should not be called for invalid tuple")
		}

	})
}
