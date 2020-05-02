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
	"math/rand"
	"sync/atomic"
	"time"
)
import "labrpc"

// import "bytes"
// import "../labgob"

//处理请求
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {

	if rf.othersHasSmallTerm(args.Term, rf.currentTerm) {
		reply.Term = rf.currentTerm
		return
	}
	rf.receiveAppendEntries <- true

	reply.Success = false
	if rf.othersHasBiggerTerm(args.Term, rf.currentTerm) {
		rf.becomeFollower(args.Term)
		reply.Term = rf.currentTerm
		return
	}

	if len(args.Entries) != 0 {
		success := rf.comparePrevLog(args.PrevLogTerm, args.PrevLogIndex)
		if success {
			rf.appendLeadersLog(args.Entries)
			reply.Success = true
			reply.NextIndex = rf.lastLogIndex() + 1
			reply.MatchIndex = rf.lastLogIndex()
			reply.AppendSuccess = true
		}
	} else {
		rf.print(LOG_HEARTBEAT, "成功处理心跳包")
		reply.Success = true
	}

	rf.updateFollowerCommitIndex(args.LeaderCommitIndex)

	reply.Term = rf.currentTerm
	return
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	rf.print(LOG_VOTE, "收到投票请求 %v", args.CandidateId)

	if rf.othersHasSmallTerm(args.Term, rf.currentTerm) {
		reply.Term = rf.currentTerm
		return
	}
	rf.receiveVoteReqs <- true

	if rf.othersHasBiggerTerm(args.Term, rf.currentTerm) {
		rf.becomeFollower(args.Term)
	}

	//2 前半句
	if rf.votedFor == -1 || rf.votedFor == args.CandidateId {
		reply.VoteGranted = true
	}

	reply.Term = rf.currentTerm

}

//发送请求
func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	if rf.role != ROLE_LEADER {
		return false
	}

	args.LeaderCommitIndex = rf.commitIndex
	args.Entries = rf.serverNextEntriesToReplica(server)

	if len(args.Entries) == 0 {
		lastLog := rf.log[rf.lastLogIndex()]
		args.PrevLogIndex = lastLog.Index
		args.PrevLogTerm = lastLog.Term
	} else {
		prevIndex := args.Entries[0].Index - 1
		args.PrevLogIndex = rf.log[prevIndex].Index
		args.PrevLogTerm = rf.log[prevIndex].Term
	}

	rf.print(LOG_HEARTBEAT, "发送心跳包给%v 当前角色:%v", server, rf.role)
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)

	if rf.othersHasBiggerTerm(reply.Term, rf.currentTerm) {
		rf.becomeFollower(reply.Term)
		return ok
	}

	if reply.AppendSuccess {
		rf.nextIndex[server] = reply.NextIndex
		rf.matchIndex[server] = reply.MatchIndex
	}

	return ok
}

// 发送请求
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {

	rf.print(LOG_VOTE, "发送 RV")
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)

	if rf.othersHasBiggerTerm(reply.Term, rf.currentTerm) {
		rf.becomeFollower(reply.Term)
		return ok
	}

	if ok {
		if reply.VoteGranted {
			rf.mu.Lock()
			rf.voteCount++
			rf.mu.Unlock()
			rf.print(LOG_VOTE, "获得选票数:%v", rf.voteCount)
		}
	}

	if rf.voteCount > len(rf.peers)/2 {
		rf.becomeLeader()
	}

	return ok
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
	// Your initialization code here (2A, 2B, 2C).

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	rf.log = append(rf.log, Entry{})
	rf.applyCh = applyCh

	DPrintf("create peer")

	go func() {
		for {
			switch rf.role {
			case ROLE_FOLLOWER:
				select {
				case <-rf.receiveAppendEntries:
				case <-rf.receiveVoteReqs:
				case <-time.After(time.Duration((rand.Int63())%1500+300) * time.Millisecond): //每个 candidate 在开始一次选举的时候会重置一个随机的选举超时时间，然后一直等待直到选举超时；这样减小了在新的选举中再次发生选票瓜分情况的可能性。
					//rf.becomeCandidate()
					rf.mu.Lock()
					rf.print(LOG_VOTE, "follower 超时,开始选举")
					rf.role = ROLE_CANDIDATE
					rf.mu.Unlock()
				}

			case ROLE_CANDIDATE:
				rf.becomeCandidate()
				select {
				case <-rf.receiveAppendEntries:
				case <-time.After(time.Duration((rand.Int63())%300+1000) * time.Millisecond):

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
			time.Sleep(20 * time.Millisecond)
		}
	}()

	go func() {
		for {
			switch rf.role {
			case ROLE_LEADER:
				rf.updateLeaderCommitStatus()

			}
			rf.tryApply()
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
	rf.mu.Lock()
	rf.role = ROLE_FOLLOWER
	rf.votedFor = -1
	rf.currentTerm = term
	rf.voteCount = 0
	rf.mu.Unlock()
	rf.print(LOG_ALL, "变成 follower 角色:%v", rf.role)
}

func (rf *Raft) becomeCandidate() {
	rf.print(LOG_ALL, "变成 candidate")
	rf.mu.Lock()
	rf.role = ROLE_CANDIDATE
	rf.currentTerm++
	rf.votedFor = rf.me
	rf.voteCount = 1
	rf.mu.Unlock()

	DPrintf("%v 开始选举 任期:%v", rf.me, rf.currentTerm)
	args := &RequestVoteArgs{
		Term:        rf.currentTerm,
		CandidateId: rf.me,
	}

	for i, _ := range rf.peers {
		if i != rf.me {
			go func(i int) {
				reply := &RequestVoteReply{}
				rf.sendRequestVote(i, args, reply)
			}(i)

		}
	}
}

func (rf *Raft) becomeLeader() {
	rf.print(LOG_ALL, "变成 leader")
	rf.mu.Lock()
	rf.role = ROLE_LEADER
	rf.votedFor = -1
	rf.voteCount = 0

	//复制阶段初始化
	rf.matchIndex = make([]int, rf.peerCount())
	rf.nextIndex = make([]int, rf.peerCount())
	for i, _ := range rf.nextIndex {
		rf.nextIndex[i] = 1
	}

	rf.mu.Unlock()
}

func (rf *Raft) peerCount() int {
	return len(rf.peers)
}

func (rf *Raft) sendHeartBeats() {

	args := &AppendEntriesArgs{
		Term:     rf.currentTerm,
		LeaderId: rf.me,
	}

	reply := &AppendEntriesReply{}
	for i, _ := range rf.peers {
		if i != rf.me {

			go rf.sendAppendEntries(i, args, reply)
		}
	}
}

func (rf *Raft) lastLogIndex() int {
	return len(rf.log) - 1
}

func (rf *Raft) appendCommand(command interface{}) int {
	index := len(rf.log) - 1 + 1
	rf.log = append(rf.log, Entry{command, rf.currentTerm, index})
	return len(rf.log) - 1
}

// --------------------- 2C ------------------- //

//
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	// Your code here (2C).
	// Example:
	// w := new(bytes.Buffer)
	// e := labgob.NewEncoder(w)
	// e.Encode(rf.xxx)
	// e.Encode(rf.yyy)
	// data := w.Bytes()
	// rf.persister.SaveRaftState(data)
}

//
// restore previously persisted state.
//
func (rf *Raft) readPersist(data []byte) {
	if data == nil || len(data) < 1 { // bootstrap without any state?
		return
	}
	// Your code here (2C).
	// Example:
	// r := bytes.NewBuffer(data)
	// d := labgob.NewDecoder(r)
	// var xxx
	// var yyy
	// if d.Decode(&xxx) != nil ||
	//    d.Decode(&yyy) != nil {
	//   error...
	// } else {
	//   rf.xxx = xxx
	//   rf.yyy = yyy
	// }
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
