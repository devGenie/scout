package raft

import (
	"io"

	"github.com/hashicorp/raft"
)

type FSM struct {
}

type snapshot struct {
}

func (fsm *FSM) Apply(*raft.Log) interface{} {
	return nil
}

func (fsm *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return &snapshot{}, nil
}

func (fsm *FSM) Restore(io.ReadCloser) error {
	return nil
}

func (s *snapshot) Persist(sink raft.SnapshotSink) error {
	return nil
}

func (s *snapshot) Release() {
	// No-op
}
