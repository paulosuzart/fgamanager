package main

import (
	"context"
	"fmt"
	"github.com/akamensky/argparse"
	openfga "github.com/openfga/go-sdk"
	"github.com/paulosuzart/fgamanager/db"
	"github.com/rivo/tview"
	"log"
	"net/url"
	"os"
)

var (
	parser  = argparse.NewParser("fgamanager", "fgamanager")
	apiUrl  = parser.String("a", "apiUrl", &argparse.Options{Default: "http://localhost:8087"})
	storeId = parser.String("s", "storeId", &argparse.Options{Required: true})
)

func init() {
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
		ApiUrl:  *apiUrl,
		StoreId: *storeId,
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
