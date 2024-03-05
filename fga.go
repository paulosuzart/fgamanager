package main

import (
	"context"
	"fmt"
	openfga "github.com/openfga/go-sdk"
	"github.com/paulosuzart/fgamanager/db"
	"log"
	"strings"
	"time"
)

func create(ctx context.Context, tupleKey string) {
	keyParts := strings.Split(tupleKey, " ")
	if len(keyParts) != 3 {
		log.Printf("Unable to create tuple %v", tupleKey)
	}
	user := keyParts[0]
	relation := keyParts[1]
	object := keyParts[2]
	key := openfga.NewTupleKey(user, relation, object)
	tuple := openfga.NewWriteRequestWrites([]openfga.TupleKey{*key})

	err := fga.write(ctx, tuple)

	if err != nil {
		log.Printf("Error writing tuple: %v", err)
		return
	}

}

func deleteMarked(ctx context.Context) {
	for {
		results := db.Repository.GetMarkedForDeletion()
		if results != nil {
			for _, tuple := range results {
				deleteTuple := openfga.TupleKeyWithoutCondition{
					User:     tuple.UserType + ":" + tuple.UserId,
					Relation: tuple.Relation,
					Object:   tuple.ObjectType + ":" + tuple.ObjectId,
				}
				deletes := []openfga.TupleKeyWithoutCondition{deleteTuple}
				resp, err := fga.delete(ctx, deletes)
				if err != nil && resp.StatusCode != 200 {
					log.Printf("Error deleting tuples %v: %v", err, resp)
				}

				if resp.StatusCode == 400 {
					log.Printf("Mark tuple as stale %v", deleteTuple)
					db.MarkStale(tuple.TupleKey)
				}
			}

		}
		time.Sleep(10 * time.Second)
	}
}

func read(ctx context.Context, watchUpdatesChan chan WatchUpdate) {
	for {
		var lastWatchUpdate *WatchUpdate
		token := db.GetContinuationToken(fgaClient.GetConfig().ApiUrl, fgaClient.GetStoreId())
		request := fgaClient.OpenFgaApi.ReadChanges(ctx).PageSize(50)
		if token != nil {
			request = request.ContinuationToken(*token)
		}
		resp, _, err := request.Execute()

		if err != nil {
			log.Printf("Failure on change fetch: %v", err)
			errStr := fmt.Sprintf("%v", err)
			if lastWatchUpdate != nil {
				lastWatchUpdate.WatchEnabled = "Error"
				watchUpdatesChan <- *lastWatchUpdate
			} else {
				lastWatchUpdate = &WatchUpdate{
					Token:        &errStr,
					Writes:       -1,
					Deletes:      -1,
					WatchEnabled: "true",
				}
				watchUpdatesChan <- *lastWatchUpdate
				continue
			}
		}

		writes := 0
		deletes := 0
		err = db.Transact(func() {
			db.UpsertConnection(db.Connection{
				ApiUrl:            fgaClient.GetConfig().ApiUrl,
				StoreId:           fgaClient.GetStoreId(),
				ContinuationToken: resp.GetContinuationToken(),
				LastSync:          time.Now(),
			})

			for _, c := range resp.GetChanges() {
				db.Repository.ApplyChange(c)
				if c.GetOperation() == openfga.WRITE {
					writes++
				} else if c.GetOperation() == openfga.DELETE {
					deletes++
				}
			}
		})
		if err != nil {
			log.Printf("Failure on change fetch: %v", err)
			errStr := fmt.Sprintf("%v", err)
			if lastWatchUpdate != nil {
				lastWatchUpdate.WatchEnabled = "Error"
				watchUpdatesChan <- *lastWatchUpdate
			} else {
				lastWatchUpdate = &WatchUpdate{
					Token:        &errStr,
					Writes:       -1,
					Deletes:      -1,
					WatchEnabled: "true",
				}
				watchUpdatesChan <- *lastWatchUpdate
				continue
			}
		}
		lastWatchUpdate = &WatchUpdate{
			Token:        token,
			Writes:       writes,
			Deletes:      deletes,
			WatchEnabled: "true",
		}
		watchUpdatesChan <- *lastWatchUpdate
		time.Sleep(2 * time.Second)
	}

}
