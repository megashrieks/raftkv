package httpapi

import (
	"net/http"
	"raftexploration/internal/fsm"

	"github.com/hashicorp/raft"
)

type API struct {
	f *fsm.FSM
	r *raft.Raft
}

func NewApi(f *fsm.FSM, r *raft.Raft) *API {
	return &API{
		f: f,
		r: r,
	}
}
func (a *API) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /key", a.getKey)
	mux.HandleFunc("POST /key", a.addKey)
	return mux
}
