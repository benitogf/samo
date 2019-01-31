package samo

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/benitogf/coat"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/rs/cors"
	"gopkg.in/godo.v2/glob"
)

// conn extends the websocket connection with a mutex
// https://godoc.org/github.com/gorilla/websocket#hdr-Concurrency
type conn struct {
	conn  *websocket.Conn
	mutex sync.Mutex
}

// pool of mode/key filtered websocket connections
type pool struct {
	key         string
	mode        string
	connections []*conn
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
	Peek(key string, now int64) (int64, int64)
}

// Storage : abstraction of persistent data layer
type Storage struct {
	Active    bool
	Separator string
	Db        Database
}

// Server : SAMO application server
type Server struct {
	mutex        sync.RWMutex
	mutexClients sync.RWMutex
	server       *http.Server
	router       *mux.Router
	clients      []*pool
	Archetypes   Archetypes
	Audit        Audit
	Storage      Database
	separator    string
	address      string
	closing      bool
	Silence      bool
	Static       bool
	console      *coat.Console
	objects      *Objects
	messages     *Messages
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

func (app *Server) makeRouteRegex() string {
	return "[a-zA-Z\\d][a-zA-Z\\d\\" + app.separator + "]+[a-zA-Z\\d]"
}

func (app *Server) checkArchetype(key string, index string, data string) bool {
	found := ""
	for ar := range app.Archetypes {
		if glob.Globexp(ar).MatchString(key) {
			found = ar
		}
	}
	if found != "" {
		return app.Archetypes[found](index, data)
	}

	return !app.Static
}

func (app *Server) waitListen() {
	var err error
	err = app.Storage.Start(app.separator)
	if err == nil {
		app.mutex.Lock()
		app.server = &http.Server{
			Addr: app.address,
			Handler: cors.New(cors.Options{
				AllowedMethods: []string{"GET", "POST", "DELETE"},
				// AllowedOrigins: []string{"http://foo.com", "http://foo.com:8080"},
				// AllowCredentials: true,
				// Debug: true,
			}).Handler(app.router)}
		app.mutex.Unlock()
		err = app.server.ListenAndServe()
		if !app.closing {
			log.Fatal(err)
		}
		return
	}

	log.Fatal(err)
}

func (app *Server) waitStart() {
	tryes := 0
	app.mutex.RLock()
	for (app.server == nil || !app.Storage.Active()) && tryes < 1000 {
		tryes++
		app.mutex.RUnlock()
		time.Sleep(10 * time.Millisecond)
		app.mutex.RLock()
	}
	app.mutex.RUnlock()
	if app.server == nil || !app.Storage.Active() {
		log.Fatal("Server start failed")
	}
	app.console.Log("glad to serve[" + app.address + "]")
}

// Start : initialize and start the http server and database connection
// 	port : service port 8800
//  host : service host "localhost"
// 	storage : path to the storage folder "data/db"
// 	separator : rune to use as key separator '/'
func (app *Server) Start(address string) {
	app.closing = false
	app.objects = &Objects{&Keys{}}
	app.address = address
	if app.separator == "" || len(app.separator) > 1 {
		app.separator = "/"
	}
	app.router = mux.NewRouter()
	app.console = coat.NewConsole(app.address, app.Silence)
	if app.Storage == nil {
		app.Storage = &MemoryStorage{
			Memdb:   make(map[string][]byte),
			Storage: &Storage{Active: false}}
	}
	if app.Audit == nil {
		app.Audit = func(r *http.Request) bool { return true }
	}
	rr := app.makeRouteRegex()
	app.router.HandleFunc("/", app.getStats)
	app.router.HandleFunc("/r/{key:"+rr+"}", app.rDel).Methods("DELETE")
	app.router.HandleFunc("/r/mo/{key:"+rr+"}", app.rPost("mo")).Methods("POST")
	app.router.HandleFunc("/r/mo/{key:"+rr+"}", app.rGet("mo")).Methods("GET")
	app.router.HandleFunc("/r/sa/{key:"+rr+"}", app.rPost("sa")).Methods("POST")
	app.router.HandleFunc("/r/sa/{key:"+rr+"}", app.rGet("sa")).Methods("GET")
	app.router.HandleFunc("/sa/{key:"+rr+"}", app.wss("sa"))
	app.router.HandleFunc("/mo/{key:"+rr+"}", app.wss("mo"))
	app.router.HandleFunc("/time", app.timeWs)
	go app.waitListen()
	app.waitStart()
	go app.timer()
}

// Close : shutdown the http server and database connection
func (app *Server) Close(sig os.Signal) {
	if !app.closing {
		app.closing = true
		app.Storage.Close()
		app.console.Err("shutdown", sig)
		if app.server != nil {
			app.server.Shutdown(context.Background())
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
