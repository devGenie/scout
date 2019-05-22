package scoutcore

import (
	"io"

	"github.com/hashicorp/raft"
)

type FSM struct {
}

func (fsm *FSM) Apply(*raft.Log) interface{} {
	return nil
}

func (fsm *FSM) Snapshot() (raft.FSMSnapshot, error) {
	return nil, nil
}

func (fsm *FSM) Restore(io.ReadCloser) error {
	return nil
}
