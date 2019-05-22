package scoutcore

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

type RaftStore struct {
	dbPath   string
	raftAddr string
	raft     *raft.Raft
	config   *raft.Config
}

func (store *RaftStore) Init() error {
	raftDB, err := raftboltdb.NewBoltStore(store.dbPath + "/raft.db")

	if err != nil {
		log.Fatal(err)
		return err
	}

	snapshotStore, err := raft.NewFileSnapshotStore(store.dbPath, 1, os.Stdout)

	if err != nil {
		log.Fatal(err)
		return err
	}

	trans, err := raft.NewTCPTransport(store.raftAddr, nil, 3, 10*time.Second, os.Stdout)
	if err != nil {
		log.Fatal(err)
		return err
	}

	store.config = raft.DefaultConfig()
	store.config.LogOutput = os.Stdout
	store.config.LocalID = raft.ServerID(store.raftAddr)

	rafter, err := raft.NewRaft(store.config, &FSM{}, raftDB, raftDB, snapshotStore, trans)

	if err != nil {
		log.Fatal(err)
		return err
	}
	fmt.Println("Finished initializing")
	store.raft = rafter

	return nil
}

func (store *RaftStore) BootstrapStore() error {
	bootstrapConfig := raft.Configuration{
		Servers: []raft.Server{
			{
				Suffrage: raft.Voter,
				ID:       store.config.LocalID,
				Address:  raft.ServerAddress(store.raftAddr),
			},
		},
	}

	raftFuture := store.raft.BootstrapCluster(bootstrapConfig)
	if err := raftFuture.Error(); err != nil {
		log.Fatal("Error bootstrapping cluster: ", err)
		return err
	}
	return nil
}
