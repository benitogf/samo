package samo

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/benitogf/coat"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"
)

// Pool : mode/key based websocket connections and watcher
type Pool struct {
	key         string
	mode        string
	connections []*websocket.Conn
}

// Audit : function to provide approval or denial of requests
type Audit func(r *http.Request) bool

// Archetype : function to check proper key->data covalent bond
type Archetype func(index string, data string) bool

// Archetypes : a map that allows structure and content formalization of key->data
type Archetypes map[string]Archetype

// Database : methods of the persistent data layer
type Database interface {
	Active() bool
	Start(separator string) error
	Close()
	Keys() ([]byte, error)
	Get(mode string, key string) ([]byte, error)
	Set(key string, index string, now int64, data string) (string, error)
	Del(key string) error
}

// Storage : abstraction of persistent data layer
type Storage struct {
	Active    bool
	Separator string
	Db        Database
}

// Server : SAMO application server
type Server struct {
	Server     *http.Server
	Router     *mux.Router
	clients    []*Pool
	Archetypes Archetypes
	Audit      Audit
	Storage    Database
	separator  string
	address    string
	console    *coat.Console
	helpers    *Helpers
	closing    bool
	Silence    bool
	Static     bool
}

// Object : data structure of elements
type Object struct {
	Created int64  `json:"created"`
	Updated int64  `json:"updated"`
	Index   string `json:"index"`
	Data    string `json:"data"`
}

// Stats : data structure of global keys
type Stats struct {
	Keys []string `json:"keys"`
}

func (app *Server) waitStart() {
	tryes := 0
	for (app.Server == nil || !app.Storage.Active()) && tryes < 100 {
		tryes++
		time.Sleep(100 * time.Millisecond)
	}
	if app.Server == nil || !app.Storage.Active() {
		log.Fatal("Server start failed")
	}
}

// Start : initialize and start the http server and database connection
// 	port : service port 8800
//  host : service host "localhost"
// 	storage : path to the storage folder "data/db"
// 	separator : rune to use as key separator '/'
func (app *Server) Start(address string) {
	app.closing = false
	app.address = address
	app.helpers = &Helpers{}
	if app.separator == "" || len(app.separator) > 1 {
		app.separator = "/"
	}
	app.Router = mux.NewRouter()
	app.console = coat.NewConsole(app.address, app.Silence)
	if app.Storage == nil {
		app.Storage = &MemoryStorage{
			Memdb:   make(map[string][]byte),
			Storage: &Storage{Active: false}}
	}
	if app.Audit == nil {
		app.Audit = func(r *http.Request) bool { return true }
	}
	rr := app.helpers.makeRouteRegex(app.separator)
	app.Router.HandleFunc("/", app.getStats)
	app.Router.HandleFunc("/r/{key:"+rr+"}", app.rDel).Methods("DELETE")
	app.Router.HandleFunc("/r/mo/{key:"+rr+"}", app.rPost("mo")).Methods("POST")
	app.Router.HandleFunc("/r/mo/{key:"+rr+"}", app.rGet("mo")).Methods("GET")
	app.Router.HandleFunc("/r/sa/{key:"+rr+"}", app.rPost("sa")).Methods("POST")
	app.Router.HandleFunc("/r/sa/{key:"+rr+"}", app.rGet("sa")).Methods("GET")
	app.Router.HandleFunc("/sa/{key:"+rr+"}", app.wss("sa"))
	app.Router.HandleFunc("/mo/{key:"+rr+"}", app.wss("mo"))
	app.Router.HandleFunc("/time", app.timeWs)
	go func() {
		var err error
		err = app.Storage.Start(app.separator)
		if err == nil {
			app.Server = &http.Server{
				Addr: app.address,
				Handler: cors.New(cors.Options{
					AllowedMethods: []string{"GET", "POST", "DELETE"},
					// AllowedOrigins: []string{"http://foo.com", "http://foo.com:8080"},
					// AllowCredentials: true,
					// Debug: true,
				}).Handler(app.Router)}
			err = app.Server.ListenAndServe()
			if !app.closing {
				log.Fatal(err)
			}
		} else {
			log.Fatal(err)
		}
	}()
	app.waitStart()
	app.console.Log("glad to serve[" + app.address + "]")
	go app.timer()
}

// Close : shutdowns the http server and database connection
func (app *Server) Close(sig os.Signal) {
	if !app.closing {
		app.closing = true
		app.Storage.Close()
		app.console.Err("shutdown", sig)
		if app.Server != nil {
			app.Server.Shutdown(context.Background())
		}
	}
}

// WaitClose : Blocks waiting for SIGINT or SIGTERM
func (app *Server) WaitClose() {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		app.Close(sig)
		done <- true
	}()
	<-done
}
