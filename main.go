package main

import (
	"context"
	"fmt"
	"github.com/akamensky/argparse"
	openfga "github.com/openfga/go-sdk"
	"github.com/paulosuzart/fgamanager/db"
	"github.com/rivo/tview"
	"log"
	"net/http"
	"net/url"
	"os"
	"testing"
)

var (
	parser  = argparse.NewParser("fgamanager", "fgamanager")
	apiUrl  = parser.String("a", "apiUrl", &argparse.Options{Default: "http://localhost:8087"})
	storeId = parser.String("s", "storeId", &argparse.Options{Required: true})
)

func init() {
	if testing.Testing() {
		testId := "TESTID"
		storeId = &testId
		return
	}
	err := parser.Parse(os.Args)
	if err != nil {
		fmt.Printf("Error: %v", err)
		os.Exit(1)
	}
	if _, err := url.Parse(*apiUrl); err != nil {
		panic("Api URL is malformed")
		os.Exit(1)
	}
}

var (
	fgaClient *openfga.APIClient
	fga       fgaService
)

type WatchUpdate struct {
	Writes, Deletes int
	Token           *string
	WatchEnabled    string
}

type fgaService interface {
	write(ctx context.Context, tuple *openfga.WriteRequestWrites) error
	delete(ctx context.Context, deletes []openfga.TupleKeyWithoutCondition) (*http.Response, error)
}

type fgaWrapper struct {
	fgaService
}

func (f *fgaWrapper) write(ctx context.Context, tuple *openfga.WriteRequestWrites) error {
	_, _, err := fgaClient.OpenFgaApi.Write(ctx).
		Body(openfga.WriteRequest{
			Writes: tuple,
		}).Execute()
	return err
}

func (f *fgaWrapper) delete(ctx context.Context, deletes []openfga.TupleKeyWithoutCondition) (*http.Response, error) {
	_, resp, err := fgaClient.OpenFgaApi.
		Write(ctx).
		Body(openfga.WriteRequest{
			Deletes: &openfga.WriteRequestDeletes{
				TupleKeys: deletes}}).Execute()
	return resp, err
}

func main() {
	// log to custom file
	LOG_FILE := "/tmp/fgamanager.log"
	// open log file
	logFile, err := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Panic(err)
	}
	defer func(logFile *os.File) {
		err := logFile.Close()
		if err != nil {

		}
	}(logFile)

	// Set log out put and enjoy :)
	log.SetOutput(logFile)

	// optional: log date-time, filename, and line number
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	db.SetupDb()
	defer db.Close()

	configuration, err := openfga.NewConfiguration(openfga.Configuration{
		ApiUrl:  *apiUrl,
		StoreId: *storeId,
	})
	fgaClient = openfga.NewAPIClient(configuration)
	fga = &fgaWrapper{}

	if err != nil {
		log.Panic("Unable to create openfga config")
	}
	app := tview.NewApplication()
	root := AddComponents(context.Background(), app)

	if err := app.SetRoot(root, true).SetFocus(root).Run(); err != nil {
		log.Panic(err)
	}

}
