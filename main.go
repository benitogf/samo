package samo

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/benitogf/coat"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

// Audit : function to define approval or denial of requests
type Audit func(r *http.Request) bool

// Server : SAMO application server
type Server struct {
	mutex       sync.RWMutex
	server      *http.Server
	Router      *mux.Router
	stream      stream
	Filters     Filters
	Audit       Audit
	Workers     int
	Subscribe   Subscribe
	Unsubscribe Unsubscribe
	Storage     Database
	address     string
	closing     int64
	active      int64
	Silence     bool
	Static      bool
	Tick        time.Duration
	console     *coat.Console
	objects     *Objects
	keys        *Keys
	messages    *Messages
}

func (app *Server) waitListen() {
	var err error
	err = app.Storage.Start()
	if err != nil {
		log.Fatal(err)
	}

	app.mutex.Lock()
	app.server = &http.Server{
		Addr: app.address,
		Handler: cors.New(cors.Options{
			AllowedMethods: []string{"GET", "POST", "DELETE", "PUT"},
			// AllowedOrigins: []string{"http://foo.com", "http://foo.com:8080"},
			// AllowCredentials: true,
			AllowedHeaders: []string{"Authorization", "Content-Type"},
			// Debug:          true,
		}).Handler(app.Router)}
	app.mutex.Unlock()
	err = app.server.ListenAndServe()
	if atomic.LoadInt64(&app.closing) != 1 {
		log.Fatal(err)
	}
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
		log.Fatal("server start failed")
	}

	if app.Storage.Watch() != nil {
		for i := 0; i < app.Workers; i++ {
			go app.broadcast(app.Storage.Watch())
		}
	}

	app.console.Log("glad to serve[" + app.address + "]")
}

func (app *Server) broadcast(sc StorageChan) {
	for {
		ev := <-sc
		go app.sendData(ev.key)
		if !app.Storage.Active() {
			break
		}
	}
}

// Start : initialize and start the http server and database connection
func (app *Server) Start(address string) {
	if atomic.LoadInt64(&app.active) == 1 {
		app.console.Err("server already active")
		return
	}

	atomic.StoreInt64(&app.active, 1)
	atomic.StoreInt64(&app.closing, 0)
	app.address = address

	if app.Router == nil {
		app.Router = mux.NewRouter()
	}

	app.console = coat.NewConsole(app.address, app.Silence)
	app.stream.console = app.console

	if app.Storage == nil {
		app.Storage = &MemoryStorage{}
	}

	if app.Tick == 0 {
		app.Tick = 1 * time.Second
	}

	if app.Audit == nil {
		app.Audit = func(r *http.Request) bool { return true }
	}

	if app.Subscribe == nil {
		app.Subscribe = func(mode string, key string, remoteAddr string) error { return nil }
	}

	if app.Unsubscribe == nil {
		app.Unsubscribe = func(mode string, key string, remoteAddr string) {}
	}
	if app.Workers == 0 {
		app.Workers = 2
	}

	app.stream.pools = append(
		app.stream.pools,
		&pool{
			key:         "time",
			mode:        "ws",
			connections: []*conn{}})

	app.stream.Subscribe = app.Subscribe
	app.stream.Unsubscribe = app.Unsubscribe
	app.Router.HandleFunc("/", app.getStats).Methods("GET")
	app.Router.HandleFunc("/{key:[a-zA-Z\\*\\d\\/]+}", app.unpublish).Methods("DELETE")
	app.Router.HandleFunc("/{key:[a-zA-Z\\*\\d\\/]+}", app.publish).Methods("POST")
	app.Router.HandleFunc("/{key:[a-zA-Z\\*\\d\\/]+}", app.read).Methods("GET")
	go app.waitListen()
	app.waitStart()
	go app.tick()
}

// Close : shutdown the http server and database connection
func (app *Server) Close(sig os.Signal) {
	if atomic.LoadInt64(&app.closing) != 1 {
		atomic.StoreInt64(&app.closing, 1)
		atomic.StoreInt64(&app.active, 0)
		app.Storage.Close()
		app.console.Err("shutdown", sig)
		if app.server != nil {
			app.server.Shutdown(context.Background())
		}
	}
}

// WaitClose : Blocks waiting for SIGINT, SIGTERM, SIGKILL, SIGHUP
func (app *Server) WaitClose() {
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGHUP)
	go func() {
		sig := <-sigs
		app.Close(sig)
		done <- true
	}()
	<-done
}
