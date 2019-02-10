# samo

[![Build Status][build-image]][build-url]


[build-url]: https://travis-ci.org/benitogf/samo
[build-image]: https://api.travis-ci.org/benitogf/samo.svg?branch=master&style=flat-square

Dynamic websocket and restful http service with a data persistence layer, it allows quick prototyping of realtime applications providing an interface with no fixed data structure or access regulations, but capable of [defining them](https://github.com/benitogf/samo#filters-and-audit) if necessary.

A very nice article with some [similar solutions](https://medium.com/@brenda.clark/firebase-alternative-3-open-source-ways-to-follow-e45d9347bc8c).

# quickstart

## client

There's a [js client library](https://www.npmjs.com/package/samo-js-client).

also a [react application example](https://github.com/benitogf/samo-js-client-example).

## server

download some [compiled release](https://github.com/benitogf/U-00A9/releases)

or with [go installed](https://golang.org/doc/install) get the library:

```bash
go get github.com/benitogf/samo
```

then create a file `main.go` with the code:
```golang
package main

import "github.com/benitogf/samo"

func main() {
	app := samo.Server{}
	app.Start("localhost:8800")
	app.WaitClose()
}
```

finally run the service with:
```bash
go run main.go
```

# Specs

| method | description | url    |
| ------------- |:-------------:| -----:|
| GET | key list | http://{host}:{port} |
| DELETE | del | http://{host}:{port}/r/{key} |

## time

server side time ticker. will send a new timestamp per second

| method | description | url    |
| ------------- |:-------------:| -----:|
| websocket| time ticker | ws://{host}:{port}/time |

## single allocation (sa)

will handle the key as key->value

| method | description | url    |
| ------------- |:-------------:| -----:|
| websocket| key data events: update, delete | ws://{host}:{port}/sa/{key} |
| POST | create/update | http://{host}:{port}/r/sa/{key} |
| GET | get object | http://{host}:{port}/r/sa/{key} |
### websocket

### get (sent after handshake and on each new/update/delete event)
---
```js
{
    created: 0,
    updated: 0,
    index: '',
    data: 'e30='
}
```

### set (format expected from client)
---
```js
{
    data: 'e30='
}
```

### del (format expected from client)
---
```js
{
    op: 'del'
}
```

## multiple objects (mo)

    will handle the key as a list of every key/[index...] (or key/*), excluding the empty index (key->value)

| method  | description | url    |
| ------------- |:-------------:| -----:|
| websocket | key data events: new, update, delete | ws://{host}:{port}/mo/{key} |
| POST | create/update, if the index is not provided it will autogenerate a new one, preexistent data on the provided key/index will be overwriten | http://{host}:{port}/r/mo |
| GET | get list | http://{host}:{port}/r/mo/{key} |

### websocket

### get (sent after handshake and on each new/update/delete event)
---
```js
[
    {
        created: 1546660572033308700,
        updated: 0,
        index: '1576d7988025d81c0',
        data: 'e30='
    }
    ...
]
```

### create/update (format expected from client)
    the index field is optional, will be autogenerated if its empty/null.
---
```js
{
    index: 'test',
    data: 'e30='
}
```

### delete (format expected from client)
---
```js
{
    op: 'del',
    index: 'test'
}
```

## filters, audit, subscription events and extra routes

    Define ad lib receive and send filter criteria using key glob patterns, audit middleware, subscription events, and extra routes

```go
package main

import (
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/benitogf/samo"
	"github.com/gorilla/mux"
)

func main() {
	app := samo.Server{}
	app.Static = true // limit to filtered paths

	// Audit requests
	app.Audit = func(r *http.Request) bool {
		return r.Method == "GET" && r.Header.Get("Upgrade") != "websocket"
	}

	// Filters
	app.ReceiveFilter("bag/*", func(index string, data []byte) ([]byte, error) {
		if string(data) != "marbles" {
			return nil, errors.New("filtered")
		}

		return data, nil
	})
	app.SendFilter("bag/1", func(index string, data []byte) ([]byte, error) {
		return []byte("intercepted"), nil
	})

	// Subscription events
	server.Subscribe = func(mode string, key string, remoteAddr string) error {
		log.Println(mode, key)
		return nil
	}
	server.Unsubscribe = func(mode string, key string, remoteAddr string) {
		log.Println(mode, key)
	}
	// Predefine the router
	app.Router = mux.NewRouter()
	app.Router.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, "123")
	})
	app.Start("localhost:8800")
	app.WaitClose()
}
```

## data persistence layer

    Use alternative storages, the default is memory

### leveldb
```go
package main

import (
	"github.com/benitogf/samo"
)

func main() {
	app := samo.Server{}
	app.Storage = &samo.LevelDbStorage{
		Path:    "data/db"}
	app.Start("localhost:8800")
	app.WaitClose()
}
```
### redis
```go
package main

import (
	"github.com/benitogf/samo"
)

func main() {
	app := samo.Server{}
	app.Storage = &samo.RedisStorage{
		Address: "localhost:6379",
		Password: ""}
	app.Start("localhost:8800")
	app.WaitClose()
}
```
### mongodb
```go
package main

import (
	"github.com/benitogf/samo"
)

func main() {
	app := samo.Server{}
	app.Storage = &samo.MongodbStorage{
		Address: "localhost:27017"}
	app.Start("localhost:8800")
	app.WaitClose()
}
```
### mariadb
```go
package main

import (
	"github.com/benitogf/samo"
)

func main() {
	app := samo.Server{}
	app.Storage = &samo.MariaDbStorage{
		User:     "root",
		Password: "",
		Name:     "test"}
	app.Start("localhost:8800")
	app.WaitClose()
}
```


