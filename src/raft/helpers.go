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

		//if len(rf.log)-1 >= N {
		//	rf.print(LOG_LEADER, "更新 leader 的 commitIndex , matchIndex:%v commitIndex%v term:%v currT:%v", rf.matchIndex, rf.commitIndex, rf.log[N].Term, rf.currentTerm)
		//}

		if rf.isMajority(num) && rf.log[N].Term == rf.currentTerm {
			rf.print(LOG_PERSIST, "更新commitIndex:", rf.log)
			rf.commitIndex = N
		}
		N++
	}
	rf.tryApply()
}

func (rf *Raft) tryApply() {

	if rf.commitIndex > rf.lastApplied {
		rf.print(LOG_PERSIST, "apply cI %v lA %v log %v", rf.commitIndex, rf.lastApplied, rf.log)
		rf.lastApplied++
		log := rf.log[rf.lastApplied]
		rf.print(LOG_PERSIST, "apply cI %v lA %v log %v", rf.commitIndex, rf.lastApplied, log)
		rf.applyCh <- ApplyMsg{true, log.Command, log.Index}
	}
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	if rf.othersHasBiggerTerm(args.Term, rf.currentTerm) {
		rf.becomeFollower(args.Term)
		reply.Term = rf.currentTerm
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

	if rf.othersHasBiggerTerm(args.Term, rf.currentTerm) {
		rf.becomeFollower(args.Term)
		reply.Term = rf.currentTerm
	}

	if rf.othersHasSmallTerm(args.Term, rf.currentTerm) {
		reply.Term = rf.currentTerm
		return
	}

	rf.reqsAERcvd <- AppendEntriesRequest{args, reply}
	<-rf.finishReqsAEHandle
	return
}

func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)

	if ok {
		if rf.othersHasBiggerTerm(reply.Term, rf.currentTerm) {
			rf.becomeFollower(reply.Term)
		}

		rf.respRVRcvd <- VoteRequest{args, reply}
	}
	return ok
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
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
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.voteFor)
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
		rf.print(LOG_REPLICA_1, "客户端发起command: isLeader:%v index:%v,term:%v", isLeader, index, term)
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
