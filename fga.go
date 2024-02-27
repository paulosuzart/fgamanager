package main

import (
	"context"
	"fmt"
	openfga "github.com/openfga/go-sdk"
	"github.com/paulosuzart/fgamanager/db"
	"log"
	"time"
)

func deleteMarked(ctx context.Context) {
	for {
		results := db.GetMarkedForDeletion()
		if results != nil {
			for _, tuple := range results {
				deleteTuple := openfga.TupleKeyWithoutCondition{
					User:     tuple.UserType + ":" + tuple.UserId,
					Relation: tuple.Relation,
					Object:   tuple.ObjectType + ":" + tuple.ObjectId,
				}
				deletes := []openfga.TupleKeyWithoutCondition{deleteTuple}
				_, resp, err := fgaClient.OpenFgaApi.
					Write(ctx).
					Body(openfga.WriteRequest{
						Deletes: &openfga.WriteRequestDeletes{
							TupleKeys: deletes}}).Execute()
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
				db.ApplyChange(c)
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
