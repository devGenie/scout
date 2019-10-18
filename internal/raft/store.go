package raft

import (
	"log"
	"os"
	"time"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

type RaftStore struct {
	dbPath          string
	raftAddr        string
	raft            *raft.Raft
	raftDB          *raftboltdb.BoltStore
	snapshotStore   *raft.FileSnapshotStore
	transport       *raft.NetworkTransport
	config          *raft.Config
	localMembership raft.Configuration
}

func (store *RaftStore) Init() error {
	raftDB, err := raftboltdb.NewBoltStore(store.dbPath + "/raft.db")
	if err != nil {
		log.Fatal(err)
		return err
	}

	store.raftDB = raftDB

	snapshotStore, err := raft.NewFileSnapshotStore(store.dbPath, 1, os.Stdout)

	if err != nil {
		log.Fatal(err)
		return err
	}

	store.snapshotStore = snapshotStore

	trans, err := raft.NewTCPTransport(store.raftAddr, nil, 3, 10*time.Second, os.Stdout)
	if err != nil {
		log.Fatal(err)
		return err
	}
	store.transport = trans

	store.config = raft.DefaultConfig()
	store.config.LogOutput = os.Stdout
	store.config.LocalID = raft.ServerID(store.raftAddr)

	rafter, err := raft.NewRaft(store.config, &FSM{}, store.raftDB, store.raftDB, store.snapshotStore, store.transport)

	if err != nil {
		log.Fatal(err)
		return err
	}
	store.raft = rafter

	store.localMembership = raft.Configuration{
		Servers: []raft.Server{
			{
				Suffrage: raft.Voter,
				ID:       store.config.LocalID,
				Address:  raft.ServerAddress(store.raftAddr),
			},
		},
	}

	return nil
}

func (store *RaftStore) BootstrapStore() error {

	raftFuture := store.raft.BootstrapCluster(store.localMembership)
	if err := raftFuture.Error(); err != nil {
		return err
	}
	return nil
}

func (store *RaftStore) Reset() error {
	err := store.transport.Close()
	if err != nil {
		return err
	}

	shutdownFuture := store.raft.Shutdown()
	if err := shutdownFuture.Error(); err != nil {
		return err
	}

	transport, err := raft.NewTCPTransport(store.raftAddr, nil, 3, 10*time.Second, os.Stdout)
	if err != nil {
		return err
	}
	store.transport = transport

	err = raft.RecoverCluster(store.config, &FSM{}, store.raftDB, store.raftDB, store.snapshotStore, store.transport, store.localMembership)
	if err != nil {
		return err
	}

	newRaft, err := raft.NewRaft(store.config, &FSM{}, store.raftDB, store.raftDB, store.snapshotStore, store.transport)
	if err != nil {
		log.Fatal(err)
		return err
	}
	store.raft = newRaft

	return nil
}
