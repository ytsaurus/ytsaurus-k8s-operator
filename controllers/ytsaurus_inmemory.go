package controllers

import (
	"net/http"
	"strconv"
	"sync"

	"go.ytsaurus.tech/yt/go/yson"
	"go.ytsaurus.tech/yt/go/yterrors"
)

type YtsaurusInMemory struct {
	mx      sync.RWMutex
	cypress map[string]any
}

func NewYtsaurusInMemory() *YtsaurusInMemory {
	return &YtsaurusInMemory{
		cypress: make(map[string]any),
	}
}

func (y *YtsaurusInMemory) Start(port int) error {
	http.HandleFunc("/api/v4/get", y.getHandler)
	http.HandleFunc("/api/v4/list", y.listHandler)
	http.HandleFunc("/api/v4/exists", y.existsHandler)
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
	values, ok := y.List(params.Path)
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
		wrapedValue := map[string][]string{"value": values}
		serialized, err = yson.Marshal(wrapedValue)
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

func (y *YtsaurusInMemory) Set(path string, value any) {
	y.mx.Lock()
	defer y.mx.Unlock()
	y.cypress[path] = value
}

func (y *YtsaurusInMemory) Get(path string) (any, bool) {
	y.mx.RLock()
	defer y.mx.RUnlock()
	value, ok := y.cypress[path]
	return value, ok
}
func (y *YtsaurusInMemory) List(path string) ([]string, bool) {
	y.mx.RLock()
	defer y.mx.RUnlock()
	value, ok := y.cypress[path]
	if !ok {
		return nil, false
	}
	var keys []string
	for key := range value.(map[string]any) {
		keys = append(keys, key)
	}
	return keys, ok
}
func (y *YtsaurusInMemory) Exists(path string) bool {
	y.mx.RLock()
	defer y.mx.RUnlock()
	_, ok := y.cypress[path]
	return ok
}
