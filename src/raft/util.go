package raft

import (
	"log"
	"strconv"
)

// Debugging
const Debug = 1

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}

const (
	LOG_ALL       = 0
	LOG_VOTE      = 1
	LOG_HEARTBEAT = 2
	LOG_REPLICA_1 = 3
	LOG_PERSIST   = 4

	LOG_LEADER = 10
)

func (rf *Raft) print(level int, format string, a ...interface{}) {
	//return
	//if
	//level != LOG_ALL &&
	//level != LOG_PERSIST {
	//	return
	//}
	m := map[int]bool{
		LOG_ALL:       true,
		LOG_VOTE:      true,
		LOG_HEARTBEAT: true,
		LOG_REPLICA_1: true,
		LOG_PERSIST:   false,
	}
	if !m[level] {
		return
	}

	format = "server " + strconv.Itoa(rf.me) + format
	DPrintf(format, a...)
}
