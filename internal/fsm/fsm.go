package fsm

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/cockroachdb/pebble"

	"github.com/hashicorp/raft"
)

type FSM struct {
	db *pebble.DB
}
type Snapshot struct {
	Data map[string]string
}

func (s *Snapshot) Persist(sink raft.SnapshotSink) error {
	data, err := json.Marshal(s.Data)
	if err != nil {
		sink.Cancel()
		return err
	}
	if _, err := sink.Write(data); err != nil {
		sink.Cancel()
		return err
	}
	return sink.Close()
}
func (s *Snapshot) Release() {}

type KVPair struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func NewFSM(db *pebble.DB) *FSM {
	return &FSM{
		db: db,
	}
}

func (f *FSM) Get(key string) (string, error) {
	value, closer, err := f.db.Get([]byte(key))
	if err != nil {
		return "", fmt.Errorf("failed to get key %s, %s", key, err)
	}
	defer closer.Close()
	return string(value), nil
}

func (f *FSM) Apply(log *raft.Log) interface{} {
	fmt.Println("Apply called: ", log.Data)

	var kv KVPair

	json.Unmarshal(log.Data, &kv)

	err := f.db.Set([]byte(kv.Key), []byte(kv.Value), pebble.Sync)
	if err != nil {
		fmt.Println("failed to set key value pair to pebble : ", err)
	}
	fmt.Println("key-value: ", kv.Key, kv.Value)

	return nil
}

func (f *FSM) Restore(rc io.ReadCloser) error {
	fmt.Println("Restore called: ", rc)
	defer rc.Close()
	data := make(map[string]string)
	if err := json.NewDecoder(rc).Decode(&data); err != nil {
		return err
	}
	for k, v := range data {
		fmt.Println("Setting ", k, ":", v)
		f.db.Set([]byte(k), []byte(v), pebble.Sync)
	}
	return nil
}

func (f *FSM) Snapshot() (raft.FSMSnapshot, error) {
	data := make(map[string]string)
	iter, err := f.db.NewIter(&pebble.IterOptions{})
	if err != nil {
		return nil, err
	}
	defer iter.Close()
	for iter.First(); iter.Valid(); iter.Next() {
		key := iter.Key()
		value := iter.Value()
		data[string(key)] = string(value)
	}
	fmt.Println("Snapshot called")
	return &Snapshot{
		Data: data,
	}, nil
}
