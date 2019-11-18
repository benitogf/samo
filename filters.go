package katamari

import (
	"errors"

	"github.com/benitogf/katamari/key"
)

// Apply filter function
// type for functions will serve as filters
// key: the key to filter
// data: the data received or about to be sent
// returns
// data: to be stored or sent to the client
// error: will prevent data to pass the filter
type apply func(key string, data []byte) ([]byte, error)

type applyHook func(key string) error

// Filter path -> match
type filter struct {
	path  string
	apply apply
}

// Before path -> match
type hook struct {
	path  string
	apply applyHook
}

// Hooks group of delete filters
type hooks []hook

// Router group of filters
type router []filter

// Filters read and write
type filters struct {
	Write  router
	Read   router
	Delete hooks
}

// https://github.com/golang/go/issues/11862

// WriteFilter add a filter that triggers on write
func (app *Server) WriteFilter(path string, apply apply) {
	app.filters.Write = append(app.filters.Write, filter{
		path:  path,
		apply: apply,
	})
}

// ReadFilter add a filter that runs before sending a read result
func (app *Server) ReadFilter(path string, apply apply) {
	app.filters.Read = append(app.filters.Read, filter{
		path:  path,
		apply: apply,
	})
}

// DeleteFilter add a filter that runs before sending a read result
func (app *Server) DeleteFilter(path string, apply applyHook) {
	app.filters.Delete = append(app.filters.Delete, hook{
		path:  path,
		apply: apply,
	})
}

// NoopFilter open noop filter
func NoopFilter(index string, data []byte) ([]byte, error) {
	return data, nil
}

// NoopHook open noop hook
func NoopHook(index string) error {
	return nil
}

// OpenFilter open noop read and write filters
func (app *Server) OpenFilter(name string) {
	app.WriteFilter(name, NoopFilter)
	app.ReadFilter(name, NoopFilter)
	app.DeleteFilter(name, NoopHook)
}

func (r router) check(path string, data []byte, static bool) ([]byte, error) {
	match := -1
	for i, filter := range r {
		if filter.path == path || key.Match(filter.path, path) {
			match = i
			break
		}
	}

	if match == -1 && !static {
		return data, nil
	}

	if match == -1 && static {
		return nil, errors.New("route not defined, static mode, key:" + path)
	}

	return r[match].apply(path, data)
}

func (r hooks) check(path string, static bool) error {
	match := -1
	for i, filter := range r {
		if filter.path == path || key.Match(filter.path, path) {
			match = i
			break
		}
	}

	if match == -1 && !static {
		return nil
	}

	if match == -1 && static {
		return errors.New("route not defined, static mode, key:" + path)
	}

	return r[match].apply(path)
}
