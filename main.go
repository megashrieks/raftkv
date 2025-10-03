package main

import (
	"fmt"
	"net/http"
	"os"
	"raftexploration/internal/fsm"
	"raftexploration/internal/httpapi"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

func newRaft(localId string, port string, state *fsm.FSM) (*raft.Raft, error) {
	config := raft.DefaultConfig()
	config.LocalID = raft.ServerID(localId)
	logStore, err := raftboltdb.NewBoltStore("./data/log-" + localId)
	stableStore := raft.NewInmemStore()
	snapshotStore, err := raft.NewFileSnapshotStore(
		"./data/snapshot-"+localId,
		2,
		os.Stdout,
	)
	if err != nil {
		fmt.Println("failed to create snapshot store : ", err)
		return nil, err
	}
	transport, err := raft.NewTCPTransport(
		"127.0.0.1:"+port,
		nil,
		3,
		time.Second*3,
		os.Stderr,
	)
	if err != nil {
		fmt.Println("failed to create transport : ", err)
		return nil, err
	}
	r, err := raft.NewRaft(
		config,
		state,
		logStore,
		stableStore,
		snapshotStore,
		transport,
	)
	if err != nil {
		fmt.Println("failed to create raft instance : ", err)
		return nil, err
	}
	fmt.Println("Raft instance created: ", r)
	return r, nil
}

func bootstrap(args []string) *raft.Configuration {

	var servers []raft.Server
	var server raft.Server
	for i, arg := range args {
		if i%2 == 0 {
			server = raft.Server{
				ID: raft.ServerID(arg),
			}
		} else {
			server.Address = raft.ServerAddress(":" + arg)
			servers = append(servers, server)
			fmt.Println("Added server: ", server)
		}
	}

	return &raft.Configuration{
		Servers: servers,
	}
}

func main() {
	args := os.Args[1:]
	localId := args[0]

	port := args[1]

	db, err := pebble.Open("./data/"+localId, &pebble.Options{})
	if err != nil {
		fmt.Println("failed to open pebble: ", err)
		return
	}
	defer db.Close()
	state := fsm.NewFSM(db)

	r, err := newRaft(localId, port, state)

	if err != nil {
		fmt.Println("Error initializing raft : ", err)
		return
	}
	configuration := bootstrap(args)

	r.BootstrapCluster(*configuration)
	var iport int
	fmt.Sscanf(port, "%d", &iport)
	iport += 1
	var sport string
	sport = fmt.Sprintf("%d", iport)
	api := httpapi.NewApi(state, r)
	http.ListenAndServe(":"+sport, api.Routes())
}
