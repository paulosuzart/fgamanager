package main

import (
	"context"
	"fmt"
	openfga "github.com/openfga/go-sdk"
	"manager/db"
	"time"
)

func read(ctx context.Context, watchUpdatesChan chan WatchUpdate) {
	for {
		var lastWatchUpdate *WatchUpdate
		token := db.GetContinuationToken(fgaClient.GetConfig().ApiUrl, fgaClient.GetStoreId())
		request := fgaClient.OpenFgaApi.ReadChanges(ctx).PageSize(100)
		if token != nil {
			request = request.ContinuationToken(*token)
		}
		resp, _, err := request.Execute()

		if err != nil {
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
