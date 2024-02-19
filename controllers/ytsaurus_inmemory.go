package controllers

// TODO(l0kix2): this code has a lot of copypaste, needs small refactoring

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yterrors"
)

type YtsaurusInMemory struct {
	mx      sync.RWMutex
	cypress map[string]any
}
type valueWithGenericAttrs struct {
	Attrs map[string]any `yson:",attrs"`
	Value any            `yson:",value"`
}

func NewYtsaurusInMemory() *YtsaurusInMemory {
	return &YtsaurusInMemory{
		cypress: map[string]any{
			"//sys/tablet_cells/":       map[string]any{},
			"//sys/tablet_cell_bundles": map[string]any{},
		},
	}
}

func (y *YtsaurusInMemory) Start(port int) error {
	http.HandleFunc("/api/v4/get", y.getHandler)
	http.HandleFunc("/api/v4/list", y.listHandler)
	http.HandleFunc("/api/v4/exists", y.existsHandler)
	http.HandleFunc("/api/v4/set", y.setHandler)
	http.HandleFunc("/api/v4/remove", y.removeHandler)
	return http.ListenAndServe("localhost:"+strconv.Itoa(port), nil)
}

func (y *YtsaurusInMemory) getHandler(w http.ResponseWriter, r *http.Request) {
	paramsSerialized := r.Header.Get("X-Yt-Parameters")
	type Params struct {
		Path string `yson:"path"`
	}
	params := Params{}
	var err error
	if r.Header.Get("X-Yt-Input-Format") == "yson" {
		err = yson.Unmarshal([]byte(paramsSerialized), &params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	value, ok := y.Get(params.Path)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		// {"code":500,"message":"Attribute \"nonexistent\" is not found","attributes":{"host":"localhost","pid":74,"tid":6385775732996512218,"fid":18446446043579299186,"datetime":"2024-02-14T11:40:14.500934Z","trace_id":"9cf2de84-837d5527-b486f4d5-98a700cf","span_id":11858309240037149274,"path":"//sys/@nonexistent"}}
		ytErr := yterrors.Error{
			Code:    500,
			Message: "Error resolving path " + params.Path,
		}
		var serializedErr []byte
		serializedErr, err = yson.Marshal(ytErr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = w.Write(serializedErr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	var serialized []byte
	if r.Header.Get("X-Yt-Output-Format") == "yson" {
		wrappedValue := map[string]any{"value": value}
		serialized, err = yson.Marshal(wrappedValue)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = w.Write(serialized)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
func (y *YtsaurusInMemory) listHandler(w http.ResponseWriter, r *http.Request) {
	paramsSerialized := r.Header.Get("X-Yt-Parameters")
	type Params struct {
		Path       string   `yson:"path"`
		Attributes []string `yson:"attributes"`
	}
	params := Params{}
	var err error
	if r.Header.Get("X-Yt-Input-Format") == "yson" {
		err = yson.Unmarshal([]byte(paramsSerialized), &params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	values, ok := y.List(params.Path, params.Attributes)
	if !ok {
		w.WriteHeader(http.StatusBadRequest)
		// {"code":500,"message":"Attribute \"nonexistent\" is not found","attributes":{"host":"localhost","pid":74,"tid":6385775732996512218,"fid":18446446043579299186,"datetime":"2024-02-14T11:40:14.500934Z","trace_id":"9cf2de84-837d5527-b486f4d5-98a700cf","span_id":11858309240037149274,"path":"//sys/@nonexistent"}}
		ytErr := yterrors.Error{
			Code:    500,
			Message: "Error resolving path " + params.Path,
		}
		var serializedErr []byte
		serializedErr, err = yson.Marshal(ytErr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = w.Write(serializedErr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		return
	}

	w.WriteHeader(http.StatusOK)
	var serialized []byte
	if r.Header.Get("X-Yt-Output-Format") == "yson" {
		wrappedValue := map[string][]valueWithGenericAttrs{"value": values}
		serialized, err = yson.Marshal(wrappedValue)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = w.Write(serialized)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
func (y *YtsaurusInMemory) existsHandler(w http.ResponseWriter, r *http.Request) {
	paramsSerialized := r.Header.Get("X-Yt-Parameters")
	type Params struct {
		Path string `yson:"path"`
	}
	params := Params{}
	var err error
	if r.Header.Get("X-Yt-Input-Format") == "yson" {
		err = yson.Unmarshal([]byte(paramsSerialized), &params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	exists := y.Exists(params.Path)

	w.WriteHeader(http.StatusOK)
	var serialized []byte
	if r.Header.Get("X-Yt-Output-Format") == "yson" {
		wrappedValue := map[string]bool{"value": exists}
		serialized, err = yson.Marshal(wrappedValue)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_, err = w.Write(serialized)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}
func (y *YtsaurusInMemory) setHandler(w http.ResponseWriter, r *http.Request) {
	paramsSerialized := r.Header.Get("X-Yt-Parameters")
	type Params struct {
		Path string `yson:"path"`
	}
	params := Params{}
	var err error
	var value any
	if r.Header.Get("X-Yt-Input-Format") == "yson" {
		err = yson.Unmarshal([]byte(paramsSerialized), &params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var bodyRaw []byte
		bodyRaw, err = io.ReadAll(r.Body)
		err = yson.Unmarshal(bodyRaw, &value)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	y.Set(params.Path, value)
	w.WriteHeader(http.StatusOK)
}
func (y *YtsaurusInMemory) removeHandler(w http.ResponseWriter, r *http.Request) {
	type Params struct {
		Path string `yson:"path"`
	}
	params := Params{}
	var err error
	if r.Header.Get("X-Yt-Input-Format") == "yson" {
		var bodyRaw []byte
		bodyRaw, err = io.ReadAll(r.Body)
		err = yson.Unmarshal(bodyRaw, &params)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	y.Remove(params.Path)
	w.WriteHeader(http.StatusOK)
}

func (y *YtsaurusInMemory) Get(path string) (any, bool) {
	y.mx.RLock()
	defer y.mx.RUnlock()
	value, ok := y.cypress[path]
	return value, ok
}
func (y *YtsaurusInMemory) List(path string, attributes []string) ([]valueWithGenericAttrs, bool) {
	y.mx.RLock()
	defer y.mx.RUnlock()
	value, ok := y.cypress[path]
	if !ok {
		return nil, false
	}
	var result []valueWithGenericAttrs
	for key := range value.(map[string]any) {
		item := valueWithGenericAttrs{Value: key}
		item.Attrs = make(map[string]any)
		for _, attr := range attributes {
			attrVal := y.cypress[path+"/"+key+"/@"+attr]
			item.Attrs[attr] = attrVal
		}
		result = append(result, item)
	}
	return result, ok
}
func (y *YtsaurusInMemory) Exists(path string) bool {
	y.mx.RLock()
	defer y.mx.RUnlock()
	_, ok := y.cypress[path]
	return ok
}
func (y *YtsaurusInMemory) Set(path string, value any) {
	y.mx.Lock()
	defer y.mx.Unlock()
	y.cypress[path] = value
	//y.updateCounters()
}
func (y *YtsaurusInMemory) Remove(path string) {
	y.mx.Lock()
	defer y.mx.Unlock()
	delete(y.cypress, path)

	lastSepIdx := strings.LastIndex(path, "/")
	parentPath := path[:lastSepIdx]
	base := path[lastSepIdx+1:]

	parent, exists := y.cypress[parentPath]
	if !exists {
		// todo: less dummy implementation must consistently maintain the tree
		panic("no " + parentPath + " for " + path)
	}
	delete(parent.(map[string]any), base)
	//y.updateCounters()
}
func (y *YtsaurusInMemory) updateCounters() {
	bundleCounters := map[string]int{}
	cellsMap, exists := y.cypress["//sys/tablet_cells"]
	if !exists {
		cellsMap = make(map[string]any)
		y.cypress["//sys/tablet_cells"] = cellsMap
	}
	for cellId := range cellsMap.(map[string]any) {
		bundle, ok := y.cypress["//sys/tablet_cells/"+cellId+"/@tablet_cell_bundle"]
		if !ok {
			panic("no @tablet_cell_bundle is set for " + cellId)
		}
		bundleCounters[bundle.(string)] += 1
	}
	for bundle, count := range bundleCounters {
		y.cypress["//sys/tablet_cell_bundles/"+bundle+"/@tablet_cell_count"] = count
	}
}
