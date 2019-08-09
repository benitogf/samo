package samo

import (
	"bufio"

	"github.com/benitogf/nsocket"
)

func (app *Server) readNs(client *nconn) {
	for {
		_, err := client.conn.Read()
		if err != nil {
			app.console.Err("readNsError["+client.conn.Path+"]", err)
			break
		}
	}
	app.stream.closeNs(client)
}

func (app *Server) serveNs() {
	for {
		newConn, err := app.nss.Server.Accept()
		if err != nil {
			app.console.Err(err)
			break
		}
		newClient := &nsocket.Client{
			Conn: newConn,
			Buf:  bufio.NewReadWriter(bufio.NewReader(newConn), bufio.NewWriter(newConn)),
		}
		// handshake message
		newClient.Path, err = newClient.Read()
		if err != nil {
			newClient.Close()
			app.console.Err("failedNsHandshake", err)
			continue
		}
		if !app.keys.IsValid(newClient.Path) {
			newClient.Close()
			app.console.Err("invalidKeyNs[" + newClient.Path + "]")
			continue
		}
		client, poolIndex := app.stream.openNs(newClient)
		// send initial msg
		cache, err := app.stream.getPoolCache(newClient.Path)
		if err != nil {
			raw, _ := app.Storage.Get(newClient.Path)
			if len(raw) == 0 {
				raw = []byte(`{ "created": 0, "updated": 0, "index": "", "data": "e30=" }`)
			}
			filteredData, err := app.Filters.Read.check(newClient.Path, raw, app.Static)
			if err != nil {
				app.stream.closeNs(client)
				client.conn.Close()
				app.console.Err("samo: filtered route", err)
				continue
			}
			app.stream.setCache(poolIndex, filteredData)
			cache = filteredData
		}
		go app.stream.writeNs(client, app.messages.encode(cache), true)
		go app.readNs(client)
	}
}
