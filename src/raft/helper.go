package raft

import "strconv"

const (
	LOG_ALL       = 0
	LOG_VOTE      = 1
	LOG_HEARTBEAT = 2
	LOG_REPLICA_1 = 3

	LOG_LEADER = 10
)

func (rf *Raft) print(level int, format string, a ...interface{}) {
	if level != LOG_ALL && level != LOG_VOTE {
		return
	}

	format = "server " + strconv.Itoa(rf.me) + format
	DPrintf(format, a...)
}
