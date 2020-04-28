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
	"sync"
	"time"
)
import "sync/atomic"
import "labrpc"

// import "bytes"
// import "../labgob"

const (
	ROLE_FOLLOWER  = 1
	ROLE_CANDIDATE = 2
	ROLE_LEADER    = 3
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
	log         []interface{}

	//volatile
	commitIndex int //log里最大的已commited的index
	lastApplied int //log里最大的index , (applied的)

	//volatile / only leader

	//other
	role                 int
	voteCount            int
	receiveAppendEntries chan bool
	receiveVoteReqs      chan bool
}

//请求结构
type AppendEntriesArgs struct {
	Term              int //当前 term
	LeaderId          int
	PrevLogIndex      int //上一个log的index
	PrevLogTerm       int //上一个log的term
	Entries           []interface{}
	LeaderCommitIndex int // ?
}

type AppendEntriesReply struct {
	Term    int
	Success bool
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

//处理请求
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {

	rf.receiveAppendEntries <- true

	reply.Success = false
	// 1
	if args.Term < rf.currentTerm {
		DPrintf("term过期请求 %v %v %v", rf.me, args, reply)
		return
	}

	reply.Success = true
	//rules for all server (reqs and response)
	if args.Term > rf.currentTerm {
		rf.mu.Lock()
		rf.currentTerm = args.Term
		rf.role = ROLE_FOLLOWER
		rf.mu.Unlock()
	}

	if args.Term == rf.currentTerm && rf.role == ROLE_CANDIDATE {
		rf.mu.Lock()
		rf.role = ROLE_FOLLOWER
		rf.mu.Unlock()
	}

	//DPrintf("心跳请求处理完毕 %v %v %v", rf.me, args, reply)
	return
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {

	rf.receiveVoteReqs <- true

	//1
	if args.Term < rf.currentTerm {
		DPrintf("term过期请求 %v %v %v", rf.me, args, reply)
		return
	}

	//rules for all server (reqs and response)
	if args.Term > rf.currentTerm {
		DPrintf("%v 角色%v 收到新的term,更新term", rf.me, rf.role)
		rf.mu.Lock()
		rf.currentTerm = args.Term
		rf.role = ROLE_FOLLOWER
		rf.votedFor = -1
		rf.mu.Unlock()
	}

	//2 前半句
	if rf.votedFor == -1 || rf.votedFor == args.CandidateId {
		reply.VoteGranted = true
	}

	DPrintf("%v的投票请求处理完毕  %v %v", args.CandidateId, args, reply)
}

//发送请求
func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	//DPrintf("发出心跳请求 %v %v %v", rf.me, args, reply)
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)

	if !reply.Success {
		return ok
	}

	//rules for all server (reqs and response)
	if reply.Term > rf.currentTerm {
		DPrintf("%v %v 收到新term", rf.me, rf.role)
		rf.mu.Lock()
		rf.currentTerm = args.Term
		rf.role = ROLE_FOLLOWER
		rf.mu.Unlock()
	}

	return ok
}

// 发送请求
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {

	DPrintf("%v发出投票请求 %v %v", rf.me, args, reply)
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)

	//rules for all server (reqs and response)
	if reply.Term > rf.currentTerm {
		rf.mu.Lock()
		rf.currentTerm = args.Term
		rf.role = ROLE_FOLLOWER
		rf.mu.Unlock()
	}

	if ok {
		if reply.VoteGranted {
			rf.mu.Lock()
			rf.voteCount++
			rf.mu.Unlock()
			DPrintf("%v 获得投票数%v", rf.me, rf.voteCount)
		}
	}

	if rf.voteCount > len(rf.peers)/2 {
		rf.mu.Lock()
		rf.role = ROLE_LEADER
		rf.mu.Unlock()
		DPrintf("%v-成为leader", rf.me)
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

	DPrintf("创建peer")

	go func() {
		for {
			switch rf.role {
			case ROLE_FOLLOWER:
				select {
				case <-rf.receiveAppendEntries:
				case <-rf.receiveVoteReqs:
				case <-time.After(time.Duration((rand.Int63())%1500+300) * time.Millisecond): //每个 candidate 在开始一次选举的时候会重置一个随机的选举超时时间，然后一直等待直到选举超时；这样减小了在新的选举中再次发生选票瓜分情况的可能性。
					DPrintf("%v follower超时->candidate ", rf.me)
					rf.mu.Lock()
					rf.role = ROLE_CANDIDATE
					rf.mu.Unlock()
				}

			case ROLE_CANDIDATE:
				rf.mu.Lock()
				rf.currentTerm++
				rf.votedFor = me
				rf.voteCount = 1
				rf.mu.Unlock()

				DPrintf("%v 开始选举 任期%v", rf.me, rf.currentTerm)
				args := &RequestVoteArgs{
					Term:        rf.currentTerm,
					CandidateId: me,
				}

				for i, _ := range rf.peers {
					if i != rf.me {
						go func(i int) {
							reply := &RequestVoteReply{}
							rf.sendRequestVote(i, args, reply)
						}(i)

					}
				}

				select {
				case <-rf.receiveAppendEntries:
					rf.mu.Lock()
					rf.role = ROLE_FOLLOWER
					rf.mu.Unlock()
				case <-time.After(time.Duration((rand.Int63())%1500+300) * time.Millisecond):
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
				args := &AppendEntriesArgs{
					Term:     rf.currentTerm,
					LeaderId: me,
				}
				reply := &AppendEntriesReply{}
				for i, _ := range rf.peers {
					if i != rf.me {

						go rf.sendAppendEntries(i, args, reply)
					}
				}

			}
			time.Sleep(20 * time.Millisecond)
		}
	}()

	return rf

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
	index := -1
	term := rf.currentTerm
	isLeader := rf.role == ROLE_LEADER

	// Your code here (2B).
	rf.log = append(rf.log, command)

	return index, term, isLeader
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
