package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	//"github.com/unidoc/unioffice/presentation"
	"github.com/360EntSecGroup-Skylar/excelize"

	gosocketio "github.com/graarh/golang-socketio"
	"github.com/graarh/golang-socketio/transport"
)

// Table holds the rows in a table
type Table struct {
	Rows [][]string `json:"Rows"`
}

// SafeTable is a table that holds Cells and is thread safe by using it's mutex
type SafeTable struct {
	Mux   sync.Mutex
	Table Table
}

func buildTable(pTable *SafeTable, excelFile *excelize.File, excelMux *sync.Mutex, sheetName string) error {
	excelMux.Lock()
	pTable.Mux.Lock()
	rows, err := excelFile.GetRows(sheetName)
	if err != nil {
		fmt.Println("Couldn't open sheet" + sheetName)
		pTable.Mux.Unlock()
		excelMux.Unlock()
		return err
	}

	pTable.Table.Rows = make([][]string, len(rows))
	for i := 0; i < len(rows); i++ {
		pTable.Table.Rows[i] = make([]string, len(rows[i]))
		copy(pTable.Table.Rows[i], rows[i])
	}

	//json, err := json.Marshal(pTable.Table)
	//fmt.Println(string(json))

	pTable.Mux.Unlock()
	excelMux.Unlock()
	return nil
}

func buildTablesLoop(finished1 chan bool, pTable *SafeTable, excelMux *sync.Mutex) {

	init := func() error {
		excelFile, err := excelize.OpenFile("Results.xlsx")
		if err != nil {
			fmt.Println(err)
			return err
		}

		buildTable(pTable, excelFile, excelMux, "Blad1")
		return nil
	}

	err := init()
	if err != nil {
		return
	}

	finished1 <- true

	for true {
		time.Sleep(5 * time.Second) // Wait 10 seconds until next table update
		excelFile, err := excelize.OpenFile("Results.xlsx")
		if err != nil {
			fmt.Println(err)
			return
		}
		buildTable(pTable, excelFile, excelMux, "Blad1")
	}
}

func runClientUpdateLoop(server *gosocketio.Server) {
	for true {
		time.Sleep(5 * time.Second)
		server.BroadcastToAll("update", "")
	}
}

func runServer(finished2 chan bool, pTable *SafeTable) {

	fmt.Println("In runServer")

	pTable.Mux.Lock()
	pTable.Mux.Unlock()

	server := gosocketio.NewServer(transport.GetDefaultWebsocketTransport())

	server.On(gosocketio.OnConnection, func(c *gosocketio.Channel) {
		log.Println("New client connected")

		c.Join("chat")
		c.Emit("update", "")
	})

	server.On("table-req", func(c *gosocketio.Channel, num int) {
		pTable.Mux.Lock()
		b, err := json.Marshal(pTable.Table)
		pTable.Mux.Unlock()
		if err != nil {
			log.Println(err)
		}
		c.Emit("table-response", string(b))
	})

	serveMux := http.NewServeMux()
	serveMux.Handle("/socket.io/", server)
	serveMux.Handle("/", http.FileServer(http.Dir("assets")))

	go runClientUpdateLoop(server)

	log.Panic(http.ListenAndServe(":80", serveMux))
	finished2 <- true
}

func main() {

	var excelMux sync.Mutex
	var safeTable SafeTable

	finished1 := make(chan bool)
	go buildTablesLoop(finished1, &safeTable, &excelMux)
	<-finished1

	finished2 := make(chan bool)
	go runServer(finished2, &safeTable)
	<-finished2
}
