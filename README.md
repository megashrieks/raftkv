# raftkv

A small distributed key-value store in Go. Writes are replicated across a cluster with the
[Raft](https://raft.github.io/) consensus algorithm ([hashicorp/raft](https://github.com/hashicorp/raft))
and each node persists its data in [Pebble](https://github.com/cockroachdb/pebble), a pure-Go
LSM storage engine. It was built as a way to learn how Raft works in practice.

## How it works

```
             POST /key                         GET /key
 client ───────────────► leader node        any node ◄─────── client
                            │                   │
                            │ raft.Apply        │ read local Pebble
                            ▼                   │
                     replicated Raft log ───────┘
                    (committed on a majority)
                            │
                            ▼
              FSM.Apply on every node → Pebble.Set
```

1. A client sends a write to the **leader**'s HTTP API.
2. The leader appends the command to the Raft log and replicates it to the followers.
3. Once a majority of nodes has stored the entry, it is committed and Raft calls `FSM.Apply` on
   **every** node, which writes the key/value into that node's Pebble database.
4. Reads are served directly from the local Pebble database of whichever node you ask.

Raft also takes periodic snapshots of the FSM (`FSM.Snapshot`) so the log can be truncated, and
uses them to bring lagging or new nodes up to date (`FSM.Restore`).

## Project layout

```
main.go                  wires everything together: storage, Raft node, cluster bootstrap, HTTP server
internal/fsm/fsm.go      raft.FSM implementation backed by Pebble (Apply / Snapshot / Restore)
internal/httpapi/        HTTP API: API struct + routes (api.go) and the key handlers (keys.go)
```

## Requirements

- Go 1.26 or newer
- No C toolchain is needed (Pebble and BoltDB are pure Go)

## Build

```sh
git clone https://github.com/megashrieks/raftkv.git
cd raftkv
go build -o raftkv .        # on Windows: go build -o raftkv.exe .
```

## Running a cluster

### Arguments

```
raftkv <self-id> <self-raft-port> [<peer-id> <peer-raft-port> ...]
```

- The **first pair** is the node being started.
- The remaining pairs are the other members of the cluster. Every node must be started with the
  same set of members (its own pair first), so they all bootstrap the same configuration.
- Each node listens for Raft traffic on `127.0.0.1:<raft-port>` and serves the HTTP API on
  **`<raft-port> + 1`**.

> **Leave a gap between Raft ports.** Because the HTTP port is the Raft port + 1, consecutive
> Raft ports (9001, 9002, 9003) collide. Use ports such as 9001, 9003, 9005.

### Three-node cluster on one machine

Open three terminals in the project directory:

```sh
# terminal 1  — Raft on 9001, HTTP on 9002
./raftkv n1 9001 n2 9003 n3 9005

# terminal 2  — Raft on 9003, HTTP on 9004
./raftkv n2 9003 n1 9001 n3 9005

# terminal 3  — Raft on 9005, HTTP on 9006
./raftkv n3 9005 n1 9001 n2 9003
```

After a second or two the nodes elect a leader. The node logs show which one won the election
(look for `entering leader state`).

A single-node "cluster" also works, which is handy for quick experiments:

```sh
./raftkv n1 9001
```

## HTTP API

### Write a key — `POST /key`

Must be sent to the **leader**. The body is JSON:

```sh
curl -X POST http://127.0.0.1:9002/key -d '{"key":"hello","value":"world"}'
```

PowerShell:

```powershell
Invoke-WebRequest http://127.0.0.1:9002/key -Method Post -Body '{"key":"hello","value":"world"}'
```

| Status | Meaning |
|--------|---------|
| `200`  | Committed by a majority and applied |
| `500`  | Not committed; the body explains why (`node is not the leader` when sent to a follower) |

If you get `node is not the leader`, retry against another node's HTTP port.

### Read a key — `GET /key?key=<name>`

Can be sent to any node:

```sh
curl "http://127.0.0.1:9004/key?key=hello"
# world
```

| Status | Meaning |
|--------|---------|
| `200`  | Body is the value |
| `404`  | Key not found (or a storage error) |

## Data on disk

Everything is written under `./data` relative to the working directory:

| Path | Contents |
|------|----------|
| `data/<id>/` | Pebble database holding the key/value state |
| `data/log-<id>` | Raft log (BoltDB) |
| `data/snapshot-<id>/` | Raft snapshots |

Restarting a node with the same arguments picks up its existing data. To start from a clean
cluster, stop all nodes and delete `./data`.

## Limitations

This is a learning project, not a production database:

- **Writes only work on the leader.** Followers reply with an error instead of forwarding the
  request or telling you who the leader is.
- **Reads can be stale.** A follower answers from its local copy, which may lag behind the
  leader. There are no linearizable reads.
- **Raft's term and vote are kept in memory** (the stable store is in-memory while the log is in
  BoltDB), so a restarted node forgets them, which weakens Raft's safety guarantees.
- **The membership is fixed at startup.** There is no API to add or remove nodes.
- **Only `set` is supported.** There is no delete.
- **Nodes only run on localhost**, and the HTTP port is always the Raft port + 1.
- Request bodies are not validated, and the `Restore` step doesn't clear keys that are missing
  from the snapshot.
