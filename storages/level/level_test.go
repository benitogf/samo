package level

import (
	"os"
	"testing"

	"github.com/benitogf/katamari"
	"github.com/benitogf/katamari/messages"
)

var units = []string{
	"\xe4\xef\xf0\xe9\xf9l\x100",
	"V'\xe4\xc0\xbb>0\x86j",
	"0'\xe40\x860",
	"\b𝅗𝅝\x85",
	"𓏝",
	"𝅅",
	"'",
	"\xd80''",
	"\xd8%''",
	"0",
	"",
}

func TestStorageLeveldb(t *testing.T) {
	t.Parallel()
	app := &katamari.Server{}
	app.Silence = true
	app.Storage = &Storage{Path: "test/db"}
	app.Start("localhost:0")
	defer app.Close(os.Interrupt)
	for i := range units {
		katamari.StorageListTest(app, t, messages.Encode([]byte(units[i])))
	}
	katamari.StorageObjectTest(app, t)
}

func TestStreamBroadcastLevel(t *testing.T) {
	t.Parallel()
	app := katamari.Server{}
	app.Silence = true
	app.ForcePatch = true
	app.NamedSocket = "ipctest1" + app.Time()
	app.Storage = &Storage{Path: "test/db1" + app.Time()}
	app.Start("localhost:0")
	app.Storage.Clear()
	defer app.Close(os.Interrupt)
	katamari.StreamBroadcastTest(t, &app)
}

func TestStreamGlobBroadcastLevel(t *testing.T) {
	t.Parallel()
	app := katamari.Server{}
	app.Silence = true
	app.ForcePatch = true
	app.NamedSocket = "ipctest2" + app.Time()
	app.Storage = &Storage{Path: "test/db2" + app.Time()}
	app.Start("localhost:0")
	app.Storage.Clear()
	defer app.Close(os.Interrupt)
	katamari.StreamGlobBroadcastTest(t, &app)
}

func TestStreamBroadcastFilter(t *testing.T) {
	t.Parallel()
	app := katamari.Server{}
	app.Silence = true
	app.ForcePatch = true
	app.NamedSocket = "ipctest3" + app.Time()
	app.Storage = &Storage{Path: "test/db3" + app.Time()}
	defer app.Close(os.Interrupt)
	katamari.StreamBroadcastFilterTest(t, &app)
}