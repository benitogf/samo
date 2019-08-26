package katamari

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/benitogf/coat"
	"github.com/benitogf/nsocket"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

// audit requests function
// will define approval or denial by the return value
// r: the request to be audited
// returns
// true: approve the request
// false: deny the request
type audit func(r *http.Request) bool

// Server application
//
// Router: can be predefined with routes and passed to be extended
//
// Audit: function to audit requests
//
// Workers: number of workers to use as readers of the storage->broadcast channel
//
// ForcePatch: flag to force patch operations
//
// OnSubscribe: function to monitor subscribe events
//
// OnUnsubscribe: function to monitor unsubscribe events
//
// Storage: database interdace implementation
//
// Silence: output silence flag
//
// Static: static routing flag
//
// Tick: time interval between ticks on the clock subscription
//
// NamedSocket: name of the ipc socket
type Server struct {
	wg            sync.WaitGroup
	server        *http.Server
	Router        *mux.Router
	stream        stream
	filters       filters
	Audit         audit
	Workers       int
	ForcePatch    bool
	OnSubscribe   subscribe
	OnUnsubscribe unsubscribe
	Storage       Database
	address       string
	closing       int64
	active        int64
	Silence       bool
	Static        bool
	Tick          time.Duration
	console       *coat.Console
	objects       *Objects
	keys          *Keys
	messages      *Messages
	nss           *nsocket.Server
	NamedSocket   string
}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.
type tcpKeepAliveListener struct {
	*net.TCPListener
}

func (app *Server) waitListen() {
	var err error
	err = app.Storage.Start()
	if err != nil {
		log.Fatal(err)
	}
	if app.NamedSocket != "" {
		app.nss, err = nsocket.NewServer(app.NamedSocket)
		if err != nil {
			log.Fatal(err)
		}
		go app.serveNs()
	}
	app.server = &http.Server{
		Addr: app.address,
		Handler: cors.New(cors.Options{
			AllowedMethods: []string{"GET", "POST", "DELETE", "PUT"},
			// AllowedOrigins: []string{"http://foo.com", "http://foo.com:8080"},
			// AllowCredentials: true,
			AllowedHeaders: []string{"Authorization", "Content-Type"},
			// Debug:          true,
		}).Handler(app.Router)}
	ln, err := net.Listen("tcp", app.address)
	if err != nil {
		log.Fatal("failed to start tcp, ", err)
	}
	atomic.StoreInt64(&app.active, 1)
	app.wg.Done()
	err = app.server.Serve(tcpKeepAliveListener{ln.(*net.TCPListener)})
	if atomic.LoadInt64(&app.closing) != 1 {
		log.Fatal(err)
	}
}

func (app *Server) waitStart() {
	if atomic.LoadInt64(&app.active) == 0 || !app.Storage.Active() {
		log.Fatal("server start failed")
	}

	// if app.Storage.Watch() != nil {
	for i := 0; i < app.Workers; i++ {
		go app.watch(app.Storage.Watch())
	}
	// }

	app.console.Log("glad to serve[" + app.address + "]")
}

func (app *Server) watch(sc StorageChan) {
	for {
		ev := <-sc
		if ev.Key != "" {
			app.console.Log("broadcast[" + ev.Key + "]")
			go app.broadcast(ev.Key)
		}
		if !app.Storage.Active() {
			break
		}
	}
}

// defaults will populate the server fields with their zero values
func (app *Server) defaults() {
	if app.Router == nil {
		app.Router = mux.NewRouter()
	}

	if app.console == nil {
		app.console = coat.NewConsole(app.address, app.Silence)
	}

	if app.stream.console == nil {
		app.stream.console = app.console
	}

	if app.Storage == nil {
		app.Storage = &MemoryStorage{}
	}

	if app.Tick == 0 {
		app.Tick = 1 * time.Second
	}

	if app.Audit == nil {
		app.Audit = func(r *http.Request) bool { return true }
	}

	if app.OnSubscribe == nil {
		app.OnSubscribe = func(key string) error { return nil }
	}

	if app.stream.OnSubscribe == nil {
		app.stream.OnSubscribe = app.OnSubscribe
	}

	if app.OnUnsubscribe == nil {
		app.OnUnsubscribe = func(key string) {}
	}

	if app.stream.OnUnsubscribe == nil {
		app.stream.OnUnsubscribe = app.OnUnsubscribe
	}

	if app.Workers == 0 {
		app.Workers = 2
	}

	app.stream.forcePatch = app.ForcePatch
	app.stream.pools = append(
		app.stream.pools,
		&pool{
			key:         "",
			connections: []*conn{}})
}

// Start : initialize and start the http server and database connection
func (app *Server) Start(address string) {
	app.address = address
	if atomic.LoadInt64(&app.active) == 1 {
		app.console.Err("server already active")
		return
	}
	atomic.StoreInt64(&app.active, 0)
	atomic.StoreInt64(&app.closing, 0)
	app.defaults()
	app.Router.HandleFunc("/", app.getStats).Methods("GET")
	app.Router.HandleFunc("/{key:[a-zA-Z\\*\\d\\/]+}", app.unpublish).Methods("DELETE")
	app.Router.HandleFunc("/{key:[a-zA-Z\\*\\d\\/]+}", app.publish).Methods("POST")
	app.Router.HandleFunc("/{key:[a-zA-Z\\*\\d\\/]+}", app.read).Methods("GET")
	app.Router.HandleFunc("/{key:[a-zA-Z\\*\\d\\/]+}", app.read).Queries("v", "{[\\d]}").Methods("GET")
	app.wg.Add(1)
	go app.waitListen()
	app.wg.Wait()
	app.waitStart()
	go app.tick()
}

// Close : shutdown the http server and database connection
func (app *Server) Close(sig os.Signal) {
	if atomic.LoadInt64(&app.closing) != 1 {
		atomic.StoreInt64(&app.closing, 1)
		atomic.StoreInt64(&app.active, 0)
		app.Storage.Close()
		if app.NamedSocket != "" {
			app.nss.Server.Close()
		}
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