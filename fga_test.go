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

func (r mockRepo) CountTuples(filter *db.Filter) int {
	return 0
}

type mockFga struct {
	fgaService
	deleteFunc func(ctx context.Context, deletes []openfga.TupleKeyWithoutCondition) (*http.Response, error)
}

func (m mockFga) delete(ctx context.Context, deletes []openfga.TupleKeyWithoutCondition) (*http.Response, error) {
	return m.deleteFunc(ctx, deletes)
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
	t.Run("Test Writes", func(t *testing.T) {
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
}
