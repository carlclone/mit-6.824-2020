package raft

import (
	"labrpc"
	"sync"
)

type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	//persistent
	currentTerm int
	votedFor    int
	log         []Entry

	//volatile
	commitIndex int
	lastApplied int

	nextIndex  []int
	matchIndex []int

	//volatile / only leader

	//other
	role                 int
	voteCount            int
	receiveAppendEntries chan bool
	receiveVoteReqs      chan bool
	applyCh              chan ApplyMsg
}

func (rf *Raft) serverNextEntriesToReplica(server int) []Entry {
	nextIndex := rf.nextIndex[server]
	var res []Entry
	if rf.lastLogIndex() >= nextIndex {
		res = rf.log[nextIndex:]
	} else {
		res = []Entry{}
	}
	rf.print(LOG_REPLICA_1, "准备复制给 server %v 的%v", server, res)

	return res
}

func (rf *Raft) comparePrevLog(prevLogTerm int, prevLogIndex int) bool {
	lastLog := rf.lastLog()
	var res bool
	if lastLog.Index == prevLogIndex && lastLog.Term == prevLogTerm {
		res = true
	} else {
		res = false
	}
	rf.print(LOG_REPLICA_1, "比对结果 %v pT:%v pI:%v,ct:%v,ci:%v", res, prevLogTerm, prevLogIndex, lastLog.Term, lastLog.Index)
	return res
}

func (rf *Raft) lastLog() Entry {
	return rf.log[rf.lastLogIndex()]
}

func (rf *Raft) appendLeadersLog(entries []Entry) {
	rf.print(LOG_REPLICA_1, "开始 append leader 给的 log,  entry:%v", entries)
	startIndex := entries[0].Index
	entriesEndIndex := entries[len(entries)-1].Index
	logEndIndex := len(rf.log) - 1
	for i := startIndex; i <= entriesEndIndex; i++ {
		entry := entries[i-startIndex]
		if i <= logEndIndex {
			if rf.log[i].Term == entry.Term {
				continue
			} else {
				rf.log[i] = entry
			}
		} else {
			rf.log = append(rf.log, entry)
		}
	}
	rf.print(LOG_REPLICA_1, "append 完毕 %v", rf.log)
}

func (rf *Raft) updateFollowerCommitIndex(leaderCommitIndex int) {
	if rf.role != ROLE_FOLLOWER {
		return
	}
	if leaderCommitIndex > rf.commitIndex {
		lastLogIndex := rf.lastLogIndex()
		if rf.commitIndex > lastLogIndex {
			rf.commitIndex = lastLogIndex
		} else {
			rf.commitIndex = leaderCommitIndex
		}
	}
	rf.print(LOG_REPLICA_1, "更新 follower commitindex 完毕 %v", rf.commitIndex)

}

func (rf *Raft) updateLeaderCommitStatus() {
	N := rf.commitIndex + 1
	num := 1
	for i, _ := range rf.matchIndex {
		if rf.matchIndex[i] >= N {
			num++
		}
	}
	if rf.isMajority(num) && rf.log[N].Term == rf.currentTerm {
		rf.commitIndex = N
	}
}

func (rf *Raft) isMajority(num int) bool {
	var res bool
	if num > rf.peerCount()/2 {
		res = true
	} else {
		res = false
	}
	return res
}

func (rf *Raft) tryApply() {
	if rf.commitIndex > rf.lastApplied {
		rf.lastApplied++
		log := rf.log[rf.lastApplied]
		rf.applyCh <- ApplyMsg{true, log.Command, log.Index}
	}
}

//请求结构
type AppendEntriesArgs struct {
	Term              int //当前 term
	LeaderId          int
	PrevLogIndex      int //
	PrevLogTerm       int
	Entries           []Entry
	LeaderCommitIndex int
}

type AppendEntriesReply struct {
	Term          int
	Success       bool
	NextIndex     int
	MatchIndex    int
	AppendSuccess bool
}

type RequestVoteArgs struct {
	// Your data here (2A, 2B).
	Term         int
	CandidateId  int
	LastLogIndex int
	LastLogTerm  int
}

type RequestVoteReply struct {
	// Your data here (2A).
	Term        int
	VoteGranted bool
}

type Entry struct {
	Command interface{}
	//任期号用来检测多个日志副本之间的不一致情况
	//Leader 在特定的任期号内的一个日志索引处最多创建一个日志条目，同时日志条目在日志中的位置也从来不会改变。该点保证了上面的第一条特性。
	Term  int
	Index int
}

//
// as each Raft peer becomes aware that successive log entries are
// committed, the peer should send an ApplyMsg to the service (or
// tester) on the same server, via the applyCh passed to Make(). set
// CommandValid to true to indicate that the ApplyMsg contains a newly
// committed log entry.
//
// in Lab 3 you'll want to send other kinds of messages (e.g.,
// snapshots) on the applyCh; at that point you can add fields to
// ApplyMsg, but set CommandValid to false for these other uses.
//
type ApplyMsg struct {
	CommandValid bool
	Command      interface{}
	CommandIndex int
}
