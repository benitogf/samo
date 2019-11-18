# katamari

[![Build Status][build-image]][build-url]
[![Documentation](https://godoc.org/github.com/benitogf/katamari?status.svg)](http://godoc.org/github.com/benitogf/katamari)

[build-url]: https://travis-ci.com/benitogf/katamari
[build-image]: https://api.travis-ci.com/benitogf/katamari.svg?branch=master&style=flat-square

![katamari](katamari.jpg)

Zero configuration data persistence and communication layer.

Web service that behaves like a distributed filesystem in the sense that all routes are open by default, oposite to rails like frameworks where the user must define the routes before being able to interact with them.

Provides a dynamic websocket and restful http service to quickly prototype realtime applications, the interface has no fixed data structure or access regulations by default, to restrict access see: [define limitations](https://github.com/benitogf/katamari#creating-rules-and-control).

## features

- dynamic routing
- glob pattern routes
- [patch](http://jsonpatch.com) updates on subscriptions
- version check on subscriptions (no message on version match)
- restful CRUD service that reflects interactions to real-time subscriptions
- named socket ipc
- storage interfaces for memory, leveldb, and etcd
- filtering and audit middleware
- auto managed timestamps (created, updated)

## quickstart

### client

There's a [js client](https://www.npmjs.com/package/katamari-client).

### server

with [go installed](https://golang.org/doc/install) get the library

```bash
go get github.com/benitogf/katamari
```

create a file `main.go`
```golang
package main

import "github.com/benitogf/katamari"

func main() {
  app := katamari.Server{}
  app.Start("localhost:8800")
  app.WaitClose()
}
```

run the service:
```bash
go run main.go
```

# routes

| method | description | url    |
| ------------- |:-------------:| -----:|
| GET | key list | http://{host}:{port} |
| websocket| clock | ws://{host}:{port} |
| POST | create/update | http://{host}:{port}/{key} |
| GET | read | http://{host}:{port}/{key} |
| DELETE | delete | http://{host}:{port}/{key} |
| websocket| subscribe | ws://{host}:{port}/{key} |

# creating rules and control

    Define ad lib receive and send filter criteria using key glob patterns, audit middleware, and extra routes

Using the default open setting is usefull while prototyping, but maybe not ideal to deploy as a public service.

jwt auth enabled with static routing server example:

```golang
package main

import (
  "net/http"
  "github.com/gorilla/mux"
  "github.com/benitogf/katamari"
  "github.com/benitogf/katamari/auth"
  "github.com/benitogf/katamari/storages/level"
)

// perform audits on the request path/headers/referer
// if the function returns false the request will return
// status 401
func audit(r *http.Request, auth *auth.TokenAuth) bool {
  if r.URL.Path == "/open" {
    return true
  }

  return auth.Verify(r)
}

func main() {
  // separated auth storage (users)
	authStore := &level.Storage{Path: "/data/auth"}
	err := authStore.Start()
	if err != nil {
		log.Fatal(err)
	}
	go katamari.WatchStorageNoop(authStore)
	auth := auth.New(
		auth.NewJwtStore(*key, time.Minute*10),
		authStore,
  )

  app := katamari.Server{}
  app.Static = true
	app.Audit = func(r *http.Request) bool {
		return router.Audit(r, auth)
  }
  app.Router = mux.NewRouter()
  katamari.OpenFilter(app, "open") // available withour token
  katamari.OpenFilter(app, "closed") // valid token required
  auth.Router(app)
  app.Start("localhost:8800")
  app.WaitClose()
}
```

### static routes

Activating this flag will limit the server to process requests defined in read and write filters

```golang
app := katamari.Server{}
app.Static = true
```


### filters

- Write filters will be called before processing a write operation
- Read filters will be called before sending the results of a read operation
- if the static flag is enabled only filtered routes will be available

```golang
app.WriteFilter("books/*", func(index string, data []byte) ([]byte, error) {
  // returning an error will deny the write
  return data, nil
})
app.ReadFilter("books/taup", func(index string, data []byte) ([]byte, error) {
  // returning an error will deny the read
  return []byte("intercepted"), nil
})
app.DeleteFilter("books/taup", func(index string) (error) {
  // returning an error will deny the delete
  return errors.New("can't delete")
})
```

### audit

```golang
app.Audit = func(r *http.Request) bool {
  return false // condition to allow access to the resource
}
```

### subscribe

```golang
// new subscription
server.OnSubscribe = func(key string) error {
  log.Println(key)
  // returning an error will deny the subscription
  return nil
}
// closing subscription
server.OnUnsubscribe = func(key string) {
  log.Println(key)
}
```

### extra routes

```golang
// Pre declare the router
app.Router = mux.NewRouter()
app.Router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
  w.Header().Set("Content-Type", "application/json")
  fmt.Fprintf(w, "123")
})
app.Start("localhost:8800")
```

### tasks

Dynamically schedule cron expresion triggered functions, there can be N schedules with different cron/data to a single function

```golang
// objects stored under writer/* with a cron expresion

// {
//    cron: '0 22 * * *',
//    text: 'gnight'
// }

// {
//    cron: '* * * * *',
//    text: 'a second went by'
// }
app.Task("writer", func(data objects.Object) {
  app.console.Log("writing", data)
})
app.Start("localhost:8800")
```

# ipc named socket

Subscribe on a separated process without websocket

### client

```go
package main

import (
	"log"

	"github.com/benitogf/nsocket"
)

func main() {
	client, err := nsocket.Dial("testns", "books/*")
	if err != nil {
		log.Fatal(err)
	}
	for {
		msg, err = client.Read()
		if err != nil {
			log.Println(err)
			break
		}
		log.Println(msg)
	}
}
```

### server

```go
package main

import "github.com/benitogf/katamari"

func main() {
  app := katamari.Server{}
  app.NamedSocket = "testns" // set this field to the name to use
  app.Start("localhost:8800")
  app.WaitClose()
}
```

# data persistence layer

    Use alternative storages (the default is memory)

### [level](https://github.com/benitogf/katamari/tree/master/level)
### [etcd](https://github.com/benitogf/katamari/tree/master/etcd)


