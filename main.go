package main

import (
	"context"
	openfga "github.com/openfga/go-sdk"
	"github.com/paulosuzart/fgamanager/db"
	"github.com/rivo/tview"
	"log"
	"os"
)

var apiUrl = "http://localhost:8087"
var storeId = ""

func init() {
	if _apiUrl := os.Getenv("API_URL"); _apiUrl != "" {
		apiUrl = _apiUrl
	}

	if _storeId := os.Getenv("STORE_ID"); _storeId != "" {
		storeId = _storeId
	} else {
		panic("Store id must be provided via STORE_ID env")
	}
}

var (
	fgaClient *openfga.APIClient
)

type WatchUpdate struct {
	Writes, Deletes int
	Token           *string
	WatchEnabled    string
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
		ApiUrl:  apiUrl,
		StoreId: storeId,
	})
	fgaClient = openfga.NewAPIClient(configuration)

	if err != nil {
		log.Panic("Unable to create openfga config")
	}
	app := tview.NewApplication()
	root := AddComponents(context.Background(), app)

	if err := app.SetRoot(root, true).SetFocus(root).Run(); err != nil {
		log.Panic(err)
	}

}
