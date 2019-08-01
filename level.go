package samo

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

// LevelStorage : composition of storage
type LevelStorage struct {
	Path    string
	client  *leveldb.DB
	mutex   sync.RWMutex
	watcher StorageChan
	*Storage
}

// Active  :
func (db *LevelStorage) Active() bool {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	return db.Storage.Active
}

// Start  :
func (db *LevelStorage) Start() error {
	var err error
	db.mutex.Lock()
	defer db.mutex.Unlock()
	if db.Storage == nil {
		db.Storage = &Storage{}
	}
	if db.Path == "" {
		db.Path = "data/db"
	}
	if db.watcher == nil {
		db.watcher = make(StorageChan)
	}
	db.client, err = leveldb.OpenFile(db.Path, nil)
	if err == nil {
		db.Storage.Active = true
	}
	return err
}

// Close  :
func (db *LevelStorage) Close() {
	db.mutex.Lock()
	defer db.mutex.Unlock()
	db.Storage.Active = false
	db.client.Close()
	close(db.watcher)
}

// Clear  :
func (db *LevelStorage) Clear() {
	iter := db.client.NewIterator(nil, nil)
	for iter.Next() {
		_ = db.client.Delete(iter.Key(), nil)
	}
	iter.Release()
}

// Keys  :
func (db *LevelStorage) Keys() ([]byte, error) {
	iter := db.client.NewIterator(nil, nil)
	stats := Stats{}
	for iter.Next() {
		stats.Keys = append(stats.Keys, string(iter.Key()))
	}
	iter.Release()
	err := iter.Error()
	if err != nil {
		return nil, err
	}

	if stats.Keys == nil {
		stats.Keys = []string{}
	}

	return db.Storage.Objects.encode(stats)
}

// Get  :
func (db *LevelStorage) Get(mode string, key string) ([]byte, error) {
	if mode == "sa" {
		data, err := db.client.Get([]byte(key), nil)
		if err != nil {
			return []byte(""), err
		}

		return data, nil
	}

	if mode == "mo" {
		globPrefixKey := strings.Split(key, "*")[0]
		rangeKey := util.BytesPrefix([]byte(globPrefixKey + ""))
		if globPrefixKey == "" || globPrefixKey == "*" {
			rangeKey = nil
		}
		iter := db.client.NewIterator(rangeKey, nil)
		res := []Object{}
		for iter.Next() {
			if db.Storage.Keys.isSub(key, string(iter.Key())) {
				newObject, err := db.Storage.Objects.decode(iter.Value())
				if err == nil {
					res = append(res, newObject)
				}
			}
		}
		iter.Release()
		err := iter.Error()
		if err != nil {
			return []byte(""), err
		}

		sort.Slice(res, db.Storage.Objects.sort(res))

		return db.Storage.Objects.encode(res)
	}

	return []byte(""), errors.New("samo: unrecognized mode: " + mode)
}

// Peek :
func (db *LevelStorage) Peek(key string, now int64) (int64, int64) {
	previous, err := db.client.Get([]byte(key), nil)
	if err != nil {
		return now, 0
	}

	oldObject, err := db.Storage.Objects.decode(previous)
	if err != nil {
		return now, 0
	}

	return oldObject.Created, now
}

// Set  :
func (db *LevelStorage) Set(key string, data string) (string, error) {
	now := time.Now().UTC().UnixNano()
	index := (&Keys{}).lastIndex(key)
	created, updated := db.Peek(key, now)
	err := db.client.Put(
		[]byte(key),
		db.Storage.Objects.new(&Object{
			Created: created,
			Updated: updated,
			Index:   index,
			Data:    data,
		}), nil)

	if err != nil {
		return "", err
	}

	db.watcher <- StorageEvent{key: key, operation: "set"}
	return index, nil
}

// Del  :
func (db *LevelStorage) Del(key string) error {
	_, err := db.client.Get([]byte(key), nil)
	if err != nil && err.Error() == "leveldb: not found" {
		return errors.New("samo: not found")
	}

	if err != nil {
		return err
	}

	err = db.client.Delete([]byte(key), nil)
	if err != nil {
		return err
	}
	db.watcher <- StorageEvent{key: key, operation: "del"}
	return nil
}

// Watch :
func (db *LevelStorage) Watch() StorageChan {
	return db.watcher
}
