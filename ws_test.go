package samo

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestWsTime(t *testing.T) {
	app := Server{}
	app.Silence = true
	app.Start("localhost:9889")
	defer app.Close(os.Interrupt)
	u := url.URL{Scheme: "ws", Host: app.address, Path: "/time"}
	c1, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	c2, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	count := 0
	c1time := ""
	c2time := ""

	go func() {
		for {
			_, message, err := c1.ReadMessage()
			if err != nil {
				app.console.Err("read c1", err)
				break
			}
			c1time = string(message)
			app.console.Log("time c1", c1time)
			count++
		}
	}()

	for {
		_, message, err := c2.ReadMessage()
		if err != nil {
			app.console.Err("read c2", err)
			break
		}
		c2time = string(message)
		app.console.Log("time c2", c2time)
		err = c2.Close()
		require.NoError(t, err)
	}

	tryes := 0
	for count < 3 && tryes < 10000 {
		tryes++
		time.Sleep(2 * time.Millisecond)
	}

	err = c1.Close()
	require.NoError(t, err)
	require.NotEmpty(t, c1time)
	require.NotEmpty(t, c2time)
}

func TestWsRestPostBroadcast(t *testing.T) {
	app := Server{}
	app.Silence = true
	app.Start("localhost:9889")
	defer app.Close(os.Interrupt)
	_ = app.Storage.Del("test")
	u := url.URL{Scheme: "ws", Host: app.address, Path: "/sa/test"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	started := false
	got := ""

	go func() {
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				app.console.Err("read c", err)
				break
			}
			data, err := (&Helpers{}).Decode(message)
			require.NoError(t, err)
			app.console.Log("read c", data)
			if started {
				got = data
				err = c.Close()
				require.NoError(t, err)
			}
			started = true
		}
	}()

	tryes := 0
	for !started && tryes < 10000 {
		tryes++
		time.Sleep(2 * time.Millisecond)
	}
	var jsonStr = []byte(`{"data":"Buy coffee and bread for breakfast."}`)
	req := httptest.NewRequest("POST", "/r/sa/test", bytes.NewBuffer(jsonStr))
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	resp := w.Result()
	body, err := ioutil.ReadAll(resp.Body)
	require.NoError(t, err)
	tryes = 0
	for got == "" && tryes < 10000 {
		tryes++
		time.Sleep(2 * time.Millisecond)
	}
	var wsObject Object
	err = json.Unmarshal([]byte(got), &wsObject)
	require.NoError(t, err)
	var rPostObject Object
	err = json.Unmarshal(body, &rPostObject)
	require.NoError(t, err)
	require.Equal(t, wsObject.Index, rPostObject.Index)
	require.Equal(t, 200, resp.StatusCode)
}

func TestWsBroadcast(t *testing.T) {
	app := Server{}
	app.Silence = true
	app.Start("localhost:9889")
	defer app.Close(os.Interrupt)
	_ = app.Storage.Del("test/MOtest")
	_ = app.Storage.Del("test/123")
	_ = app.Storage.Del("test/1")
	index, err := app.Storage.Set("test/1", "1", time.Now().UTC().UnixNano(), "test")
	require.NoError(t, err)
	require.Equal(t, "1", index)

	index, err = app.Storage.Set("test/2", "2", time.Now().UTC().UnixNano(), "test")
	require.NoError(t, err)
	require.Equal(t, "2", index)

	u1 := url.URL{Scheme: "ws", Host: app.address, Path: "/mo/test"}
	u2 := url.URL{Scheme: "ws", Host: app.address, Path: "/sa/test/1"}
	c1, _, err := websocket.DefaultDialer.Dial(u1.String(), nil)
	require.NoError(t, err)
	c2, _, err := websocket.DefaultDialer.Dial(u2.String(), nil)
	require.NoError(t, err)
	wrote := false
	readCount := 0
	got1 := ""
	got2 := ""

	go func() {
		for {
			_, message, err := c1.ReadMessage()
			if err != nil {
				app.console.Err("read c1", err)
				break
			}
			data, err := (&Helpers{}).Decode(message)
			require.NoError(t, err)
			app.console.Log("read c1", data)
			if readCount == 2 {
				got1 = data
				err = c1.Close()
				require.NoError(t, err)
			}
			if readCount == 0 {
				err = c1.WriteMessage(websocket.TextMessage, []byte("{"+
					"\"op\": \"del\","+
					"\"index\": \"2\""+
					"}"))
				require.NoError(t, err)
			}
			readCount++
		}
	}()

	tryes := 0
	for readCount < 2 && tryes < 10000 {
		tryes++
		time.Sleep(2 * time.Millisecond)
	}

	for {
		_, message, err := c2.ReadMessage()
		if err != nil {
			app.console.Err("read", err)
			break
		}
		data, err := (&Helpers{}).Decode(message)
		require.NoError(t, err)
		app.console.Log("read c2", data)
		if wrote {
			got2 = data
			err = c2.Close()
			require.NoError(t, err)
		} else {
			app.console.Log("writing from c2")
			err = c2.WriteMessage(websocket.TextMessage, []byte("{"+
				"\"index\": \"1\","+
				"\"data\": \"test2\""+
				"}"))
			require.NoError(t, err)
			wrote = true
		}
	}

	tryes = 0
	for got1 == "" && tryes < 10000 {
		tryes++
		time.Sleep(2 * time.Millisecond)
	}

	require.Equal(t, got1, "["+got2+"]")
}

func TestWsDel(t *testing.T) {
	app := Server{}
	app.Silence = true
	app.Start("localhost:9889")
	defer app.Close(os.Interrupt)
	_ = app.Storage.Del("test")
	index, err := app.Storage.Set("test", "test", time.Now().UTC().UnixNano(), "test")
	require.NoError(t, err)
	require.NotEmpty(t, index)
	u := url.URL{Scheme: "ws", Host: app.address, Path: "/sa/test"}
	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	require.NoError(t, err)
	started := false

	for {
		_, message, err := c.ReadMessage()
		if err != nil {
			app.console.Err("read c", err)
			break
		}
		data, err := (&Helpers{}).Decode(message)
		require.NoError(t, err)
		app.console.Log("read c", data)
		if started {
			err = c.Close()
			require.NoError(t, err)
		} else {
			err = c.WriteMessage(websocket.TextMessage, []byte("{"+
				"\"op\": \"del\""+
				"}"))
			require.NoError(t, err)
			started = true
		}
	}

	req := httptest.NewRequest("GET", "/r/sa/test", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	resp := w.Result()

	require.Equal(t, 404, resp.StatusCode)
}

func TestWsBadRequest(t *testing.T) {
	app := Server{}
	app.Silence = true
	app.Start("localhost:9889")
	defer app.Close(os.Interrupt)

	req := httptest.NewRequest("GET", "/sa/test", nil)
	w := httptest.NewRecorder()
	app.Router.ServeHTTP(w, req)
	resp := w.Result()

	require.Equal(t, 400, resp.StatusCode)
}
