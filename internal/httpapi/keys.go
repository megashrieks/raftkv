package httpapi

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

func (a *API) getKey(w http.ResponseWriter, req *http.Request) {
	key := req.URL.Query().Get("key")
	value, err := a.f.Get(key)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(err.Error()))
		return
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(value))

}

func (a *API) addKey(w http.ResponseWriter, req *http.Request) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}
	if err := a.r.Apply(body, time.Second*3).Error(); err != nil {
		fmt.Println("Failed to apply: ", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	w.WriteHeader(http.StatusOK)
}
