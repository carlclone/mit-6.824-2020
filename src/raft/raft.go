package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(command interface{}) (index, term, isleader)
//   start agreement on a new log entry
// rf.GetState() (term, isLeader)
//   ask a Raft for its current term, and whether it thinks it is leader
// ApplyMsg
//   each time a new entry is committed to the log, each Raft peer
//   should send an ApplyMsg to the service (or tester)
//   in the same server.
//

import (
	"bytes"
	"labgob"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)
import "labrpc"

// import "bytes"
// import "../labgob"

const (
	ROLE_FOLLOWER  = 1
	ROLE_CANDIDATE = 2
	ROLE_LEADER    = 3
)

type Raft struct {
	mu        sync.Mutex // Lock to protect shared access to this peer's state
	mu2       sync.Mutex
	mu3       sync.Mutex
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
	role      int
	voteCount int

	receiveAppendEntries chan bool
	receiveVoteReqs      chan bool
	applyCh              chan ApplyMsg
}

func (rf *Raft) GetState() (int, bool) {

	// Your code here (2A).
	return rf.currentTerm, rf.role == ROLE_LEADER
}

func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me

	rf.receiveAppendEntries = make(chan bool, 50)
	rf.receiveVoteReqs = make(chan bool, 50)

	rf.votedFor = -1
	rf.role = ROLE_FOLLOWER
	rf.log = append(rf.log, Entry{})
	// Your initialization code here (2A, 2B, 2C).

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	rf.applyCh = applyCh

	DPrintf("create peer")

	go func() {
		for {
			switch rf.role {
			case ROLE_FOLLOWER:
				select {
				case <-rf.receiveAppendEntries:
				case <-rf.receiveVoteReqs:
				case <-time.After(time.Duration((rand.Int63())%500+200) * time.Millisecond): //每个 candidate 在开始一次选举的时候会重置一个随机的选举超时时间，然后一直等待直到选举超时；这样减小了在新的选举中再次发生选票瓜分情况的可能性。
					//rf.becomeCandidate()
					rf.mu.Lock()
					rf.print(LOG_VOTE, "follower 超时,开始选举")
					rf.role = ROLE_CANDIDATE
					rf.becomeCandidate()
					rf.mu.Unlock()
				}

			case ROLE_CANDIDATE:

				select {
				case <-rf.receiveAppendEntries:
				case <-time.After(time.Duration((rand.Int63())%500+200) * time.Millisecond):

				}
			case ROLE_LEADER:
				time.Sleep(20 * time.Millisecond)
			}
		}
	}()

	go func() {
		for {
			switch rf.role {
			case ROLE_LEADER:
				rf.sendHeartBeats()
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	go func() {
		for {
			rf.mu2.Lock()
			switch rf.role {
			case ROLE_LEADER:
				rf.updateLeaderCommitStatus()

			}
			rf.tryApply()
			rf.mu2.Unlock()
			time.Sleep(20 * time.Millisecond)
		}
	}()

	return rf

}

func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	index := -1
	term := -1
	isLeader := rf.isLeader()

	if isLeader {
		index = rf.appendCommand(command)
		term = rf.currentTerm
		rf.print(LOG_REPLICA_1, "客户端发起command:%v isLeader:%v index:%v,term:%v", command, isLeader, index, term)
	}

	return index, term, isLeader
}

// ------------------------ helpers -----------------------

func (rf *Raft) othersHasSmallTerm(othersTerm int, term int) bool {
	if othersTerm < term {
		rf.print(LOG_ALL, "收到过期 term other:%v curr:%v", othersTerm, term)
	}

	return othersTerm < term
}

func (rf *Raft) othersHasBiggerTerm(othersTerm int, currentTerm int) bool {
	if othersTerm > currentTerm {
		rf.print(LOG_ALL, "收到更大的 term  other%v curr%v", othersTerm, currentTerm)
	}
	return othersTerm > currentTerm
}

func (rf *Raft) isLeader() bool {
	return rf.role == ROLE_LEADER
}

func (rf *Raft) becomeFollower(term int) {
	rf.role = ROLE_FOLLOWER
	rf.votedFor = -1
	rf.currentTerm = term
	rf.persist()
	rf.voteCount = 0
	rf.print(LOG_ALL, "变成 follower 角色:%v", rf.role)
}

func (rf *Raft) becomeCandidate() {
	if rf.role == ROLE_CANDIDATE {
		rf.print(LOG_ALL, "candidate新一轮选举")
	} else {
		rf.print(LOG_ALL, "变成 candidate")
	}

	rf.role = ROLE_CANDIDATE
	rf.currentTerm++
	rf.votedFor = rf.me
	//rf.persist()
	rf.voteCount = 1

	rf.print(LOG_VOTE, "开始选举,任期:%v", rf.currentTerm)

	for i, _ := range rf.peers {
		if i != rf.me {
			go func(i int) {
				args := &RequestVoteArgs{
					Term:        rf.currentTerm,
					CandidateId: rf.me,
				}
				lastLog := rf.lastLog()
				args.LastLogTerm = lastLog.Term
				args.LastLogIndex = lastLog.Index
				reply := &RequestVoteReply{}
				rf.sendRequestVote(i, args, reply)
			}(i)

		}
	}
}

func (rf *Raft) becomeLeader() {
	rf.print(LOG_ALL, "变成 leader")
	rf.role = ROLE_LEADER
	rf.votedFor = -1
	rf.persist()
	rf.voteCount = 0

	//复制阶段初始化
	rf.matchIndex = make([]int, rf.peerCount())
	rf.nextIndex = make([]int, rf.peerCount())
	for i, _ := range rf.nextIndex {
		rf.nextIndex[i] = rf.lastLogIndex() + 1
	}

}

func (rf *Raft) peerCount() int {
	return len(rf.peers)
}

func (rf *Raft) sendHeartBeats() {

	for i, _ := range rf.peers {
		if i != rf.me {
			go func(i int) {
				args := &AppendEntriesArgs{
					Term:     rf.currentTerm,
					LeaderId: rf.me,
				}

				reply := &AppendEntriesReply{}
				rf.sendAppendEntries(i, args, reply)
			}(i)

		}
	}
}

func (rf *Raft) lastLogIndex() int {
	return len(rf.log) - 1
}

func (rf *Raft) appendCommand(command interface{}) int {
	index := len(rf.log) - 1 + 1
	rf.log = append(rf.log, Entry{command, rf.currentTerm, index})
	rf.persist()
	return len(rf.log) - 1
}

// --------------------- 2C ------------------- //

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.votedFor)
	e.Encode(rf.currentTerm)
	e.Encode(rf.log)
	data := w.Bytes()
	rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	r := bytes.NewBuffer(data)
	d := labgob.NewDecoder(r)
	var votedFor int
	var currentTerm int
	var log []Entry
	if d.Decode(&votedFor) != nil ||
		d.Decode(&currentTerm) != nil ||
		d.Decode(&log) != nil {
		//error...
		rf.print(LOG_ALL, "decode出错,或为空")
	} else {
		rf.votedFor = votedFor
		rf.currentTerm = currentTerm
		rf.log = log
	}
}

//
// the tester doesn't halt goroutines created by Raft after each test,
// but it does call the Kill() method. your code can use killed() to
// check whether Kill() has been called. the use of atomic avoids the
// need for a lock.
//
// the issue is that long-running goroutines use memory and may chew
// up CPU time, perhaps causing later tests to fail and generating
// confusing debug output. any goroutine with a long-running loop
// should call killed() to check whether it should stop.
//
func (rf *Raft) Kill() {
	atomic.StoreInt32(&rf.dead, 1)
	// Your code here, if desired.
}

func (rf *Raft) killed() bool {
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

func (rf *Raft) serverNextEntriesToReplica(server int) []Entry {
	nextIndex := rf.nextIndex[server]
	var res []Entry
	if rf.lastLogIndex() >= nextIndex {
		res = rf.log[nextIndex:]
	} else {
		res = []Entry{}
	}
	if len(res) != 0 {
		rf.print(LOG_REPLICA_1, "准备复制给 server %v 的%v nI%v mI%v", server, res, rf.nextIndex[server], rf.matchIndex[server])
	}

	return res
}

func (rf *Raft) comparePrevLog(prevLogTerm int, prevLogIndex int) bool {
	if prevLogIndex > len(rf.log)-1 {
		return false
	}

	prevlog := rf.log[prevLogIndex]
	var res bool
	if prevlog.Index == prevLogIndex && prevlog.Term == prevLogTerm {
		res = true
	} else {
		res = false
	}
	//rf.print(LOG_REPLICA_1, "比对结果 %v pT:%v pI:%v,ct:%v,ci:%v", res, prevLogTerm, prevLogIndex, prevlog.Term, prevlog.Index)
	return res
}

func (rf *Raft) lastLog() Entry {
	return rf.log[rf.lastLogIndex()]
}

func (rf *Raft) appendLeadersLog(entries []Entry) {
	rf.print(LOG_REPLICA_1, "开始 append leader 给的 log,  entry:%v", entries)
	startIndex := entries[0].Index
	//entriesEndIndex := entries[len(entries)-1].Index

	rf.log = rf.log[:startIndex]
	//logEndIndex := len(rf.log) - 1
	rf.log = append(rf.log, entries...)
	//for i := startIndex; i <= entriesEndIndex; i++ {
	//	entry := entries[i-startIndex]
	//	if i <= logEndIndex {
	//		if rf.log[i].Term == entry.Term {
	//			continue
	//		} else {
	//			rf.log[i] = entry
	//		}
	//	} else {
	//		rf.log = append(rf.log, entry)
	//	}
	//}
	rf.persist()
	rf.print(LOG_REPLICA_1, "append 完毕 %v", rf.log)
}

func (rf *Raft) updateFollowerCommitIndex(leaderCommitIndex int) {
	if rf.role != ROLE_FOLLOWER {
		return
	}
	if leaderCommitIndex > rf.commitIndex {
		lastLogIndex := rf.lastLogIndex()
		if leaderCommitIndex > lastLogIndex {
			rf.commitIndex = lastLogIndex
		} else {
			rf.commitIndex = leaderCommitIndex
		}
	}
	rf.print(LOG_PERSIST, "更新 follower commitindex 完毕 %v", rf.commitIndex)

}

func (rf *Raft) updateLeaderCommitStatus() {
	N := rf.commitIndex + 1
	for N <= rf.lastLogIndex() {
		num := 1
		for i, _ := range rf.matchIndex {
			if rf.matchIndex[i] >= N {
				num++
			}
		}

		//if len(rf.log)-1 >= N {
		//	rf.print(LOG_LEADER, "更新 leader 的 commitIndex , matchIndex:%v commitIndex%v term:%v currT:%v", rf.matchIndex, rf.commitIndex, rf.log[N].Term, rf.currentTerm)
		//}

		if rf.isMajority(num) && rf.log[N].Term == rf.currentTerm {
			rf.print(LOG_PERSIST, "达到大多数 %v", rf.log)
			rf.commitIndex = N
		}
		N++
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
		rf.print(LOG_PERSIST, "尝试 apply cI %v lA %v log %v", rf.commitIndex, rf.lastApplied, rf.log)
		rf.lastApplied++
		log := rf.log[rf.lastApplied]
		rf.applyCh <- ApplyMsg{true, log.Command, log.Index}
	}
}

//Raft 通过比较两份日志中最后一条日志条目的索引值和任期号来定义谁的日志比较新。
// 如果两份日志最后条目的任期号不同，那么任期号大的日志更新。
// 如果两份日志最后条目的任期号相同，那么日志较长的那个更新。
func (rf *Raft) isNewestLog(lastLogIndex int, lastLogTerm int) bool {
	lastLog := rf.lastLog()
	if lastLogTerm > lastLog.Term {
		return true
	}
	if lastLogTerm == lastLog.Term && lastLogIndex >= lastLog.Index {
		return true
	}
	return false
}

func (rf *Raft) findFirstIndexOfTerm(prevLogTerm int) int {
	for _, entry := range rf.log {
		if entry.Term == prevLogTerm {
			return entry.Index
		}
	}
	return -1
}
