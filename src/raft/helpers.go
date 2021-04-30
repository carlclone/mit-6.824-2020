package raft

import (
	"bytes"
	"labgob"
	"math/rand"
	"sync/atomic"
)

func (rf *Raft) isMajority(num int) bool {
	var res bool
	if num > rf.peerCount/2 {
		res = true
	} else {
		res = false
	}
	return res
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

		if rf.isMajority(num) && rf.log[N].Term == rf.currentTerm {
			rf.print(LOG_PERSIST, "更新commitIndex:", rf.log)
			rf.commitIndex = N
		}
		N++
	}

}

func (rf *Raft) tryApply() {

	for rf.commitIndex > rf.lastApplied {
		rf.print(LOG_PERSIST, "apply cI %v lA %v log %v", rf.commitIndex, rf.lastApplied, rf.log)
		rf.lastApplied++
		log := rf.log[rf.lastApplied]
		rf.print(LOG_PERSIST, "apply cI %v lA %v log %v", rf.commitIndex, rf.lastApplied, log)
		rf.applyCh <- ApplyMsg{true, log.Command, log.Index, TYPE_NORMAL}
	}
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	if rf.othersHasBiggerTerm(args.Term, rf.currentTerm) {
		rf.becomeFollower(args.Term)
		reply.Term = args.Term
	}

	if rf.othersHasSmallTerm(args.Term, rf.currentTerm) {
		reply.Term = rf.currentTerm
		return
	}

	rf.reqsRVRcvd <- VoteRequest{args, reply}
	<-rf.finishReqsRVHandle
	return

}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {

	rf.print(LOG_ALL, "收到心跳包")

	if rf.othersHasBiggerTerm(args.Term, rf.currentTerm) {
		rf.print(LOG_ALL, "problem4_")
		rf.becomeFollower(args.Term)
		reply.Term = args.Term
	}

	if rf.othersHasSmallTerm(args.Term, rf.currentTerm) {
		rf.print(LOG_ALL, "problem5_")
		reply.Term = rf.currentTerm
		return
	}

	rf.print(LOG_ALL, "problem1_")
	rf.reqsAERcvd <- AppendEntriesRequest{args, reply}
	rf.print(LOG_ALL, "problem2_")
	<-rf.finishReqsAEHandle
	rf.print(LOG_ALL, "problem3_")
	return
}

func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)

	if ok {
		if rf.othersHasBiggerTerm(reply.Term, rf.currentTerm) {
			rf.becomeFollower(reply.Term)
		}

		if reply.VoteGranted {
			rf.respRVRcvd <- VoteRequest{args, reply}
		}
	}
	return ok
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	rf.print(LOG_ALL, "sendAe_ ")
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	rf.print(LOG_ALL, "sendAe OK:%v", ok)
	if ok {
		rf.respAERcvd <- AppendEntriesRequest{args, reply}
	}
	return ok
}

func (rf *Raft) electionTimeOut() int {
	return int(rand.Int63())%500 + 200
	//return time.Duration((rand.Int63())%500+200) * time.Millisecond
}

// ----------------------- stable ---------------------------- //

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	//rf.print(LOG_ALL, "is leader %v", rf.role == ROLE_LEADER)
	return rf.currentTerm, rf.role == ROLE_LEADER
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
	} else {
		rf.voteFor = votedFor
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
	rf.print(LOG_ALL, "killed_")
	z := atomic.LoadInt32(&rf.dead)
	return z == 1
}

//
// the service using Raft (e.g. a k/v server) wants to start
// agreement on the next command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the command will appear at
// if it's ever committed. the second return value is the current
// term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	index := -1
	term := -1
	isLeader := rf.isLeader()

	if isLeader {
		index = rf.appendCommand(command)
		term = rf.currentTerm
		rf.print(LOG_REPLICA_1, "客户端发起command:%v isLeader:%v index:%v,term:%v ", command, isLeader, index, term)
	}

	return index, term, isLeader
}

func (rf *Raft) isLeader() bool {
	return rf.role == ROLE_LEADER
}

func (rf *Raft) appendCommand(command interface{}) int {
	replicatedIndex := len(rf.log) - 1
	nextIndex := replicatedIndex + 1
	rf.log = append(rf.log, Entry{command, rf.currentTerm, nextIndex})
	rf.persist()
	return nextIndex
}
