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
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)
import "labrpc"

// import "bytes"
// import "../labgob"

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

const (
	ROLE_LEADER    = 1
	ROLE_CANDIDATE = 2
	ROLE_FOLLOWER  = 3
)

type Entry struct {
	Command interface{}
	/* 任期号用来检测多个日志副本之间的不一致情况
	 * Leader 在特定的任期号内的一个日志索引处最多创建一个日志条目，同时日志条目在日志中的位置也从来不会改变。该点保证了上面的第一条特性。
	 */
	Term  int //
	Index int
}

type Request struct {
	args  interface{}
	reply interface{}
}

type VoteRequest struct {
	args  *RequestVoteArgs
	reply *RequestVoteReply
}

type AppendEntriesRequest struct {
	args  *AppendEntriesArgs
	reply *AppendEntriesReply
}

//
// A Go object implementing a single Raft peer.
//
type Raft struct {
	mu        sync.Mutex          // Lock to protect shared access to this peer's state
	peers     []*labrpc.ClientEnd // RPC end points of all peers
	persister *Persister          // Object to hold this peer's persisted state
	me        int                 // this peer's index into peers[]
	dead      int32               // set by Kill()

	role int //角色

	//持久化的
	currentTerm int //理解为logical clock , 当前的任期
	voteFor     int //投票对象
	log         []Entry

	//volatile
	commitIndex int //当前已提交的最后一个index
	lastApplied int //最后一个通知客户端完成复制的index

	//leader属性
	nextIndex  []int //下一个要发送给peers的entry的index , 用于定位 peer和leader日志不一致的范围
	matchIndex []int //peer们最后一个确认复制的index ,用于apply

	receiveRequestVote        chan VoteRequest
	requestVoteHandleFinished chan bool

	receiveAppendEntries        chan AppendEntriesRequest
	appendEntriesHandleFinished chan bool

	startNewElection            chan bool
	concurrentSendVote          chan bool
	concurrentSendAppendEntries chan bool
	voteCount                   int
	electionTimer               Timer
	heartBeatTimer              Timer
	someOneVoted                chan bool
	peerCount                   int
}

type RequestVoteArgs struct {
	Term        int
	CandidateId int
}

type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

type AppendEntriesArgs struct {
	Term     int
	LeaderId int
}

type AppendEntriesReply struct {
	Term int
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	rf.receiveRequestVote <- VoteRequest{args, reply}

	<-rf.requestVoteHandleFinished
	rf.print(LOG_ALL, "投票请求处理完毕 %v", reply.VoteGranted)
	return

}

func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)

	rf.print(LOG_ALL, "收到的投票结果 %v", reply.VoteGranted)

	if ok {
		acceptable := rf.voteCommonResponseHandler(VoteRequest{args, reply})
		if !acceptable {
			rf.print(LOG_ALL, "unacceptable")
			return ok
		}
		if reply.VoteGranted {
			rf.print(LOG_ALL, "收到支持投票")
			rf.someOneVoted <- true
		}

	}
	return ok
}

func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.receiveAppendEntries <- AppendEntriesRequest{args, reply}
	<-rf.appendEntriesHandleFinished
	return
}

func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	return ok
}

// Make() must return quickly, so it should start goroutines for any long-running work.
func Make(peers []*labrpc.ClientEnd, me int,
	persister *Persister, applyCh chan ApplyMsg) *Raft {
	rf := &Raft{}
	rf.peers = peers
	rf.persister = persister
	rf.me = me
	rf.peerCount = len(rf.peers)
	rf.role = ROLE_FOLLOWER
	rf.voteFor = -1

	rf.electionTimer = Timer{stopped: true, timeoutMsGenerator: rf.electionTimeOut}
	rf.heartBeatTimer = Timer{stopped: true, timeoutMsGenerator: func() int {
		return 20
	}}

	rf.startNewElection = make(chan bool)
	rf.someOneVoted = make(chan bool, 50)
	rf.requestVoteHandleFinished = make(chan bool)
	rf.appendEntriesHandleFinished = make(chan bool)

	rf.concurrentSendVote = make(chan bool)
	rf.concurrentSendAppendEntries = make(chan bool)

	rf.receiveRequestVote = make(chan VoteRequest, 50)
	rf.receiveAppendEntries = make(chan AppendEntriesRequest, 50)
	rf.someOneVoted = make(chan bool, 50)

	/*
	 * 事件流向 : 定时器线程,请求,响应产生事件,  主事件线程接收 ,处理 , 如果有需要并发的(网络请求),分配并发事件到并发线程
	 */
	rf.electionTimer.start()
	//主事件循环线程
	go func() {
		for {
			switch rf.role {
			case ROLE_LEADER:
				rf.heartBeatTimer.start()
				select {
				//处理心跳包
				case <-rf.receiveAppendEntries:
				//处理投票
				case <-rf.receiveRequestVote:
				}
			case ROLE_CANDIDATE:
				select {

				case <-rf.someOneVoted:
					rf.print(LOG_ALL, "vote++")
					rf.voteCount++
					if rf.voteCount > len(peers)/2 {
						rf.print(LOG_ALL, "成为leader")
						rf.becomeLeader()
					}
				case <-rf.startNewElection:
					rf.concurrentSendVote <- true
				//处理心跳包
				case request := <-rf.receiveAppendEntries:
					rf.becomeFollower(request.args.Term)
					rf.appendEntriesHandleFinished <- true
				//处理投票
				case <-rf.receiveRequestVote:

				//选举超时,新一轮选举
				case <-rf.startNewElection:
					rf.becomeCandidate()
				}
			case ROLE_FOLLOWER:

				rf.print(LOG_VOTE, " follower初始")
				select {
				//开始选举
				case <-rf.startNewElection:
					rf.becomeCandidate()
				//收到心跳包
				case request := <-rf.receiveAppendEntries:
					rf.print(LOG_ALL, "收到心跳包!!!!!!!!!!!!!!")
					//公共处理,并判断是否继续处理该请求
					acceptable := rf.appendEntriesCommonHandler(request)
					if !acceptable {
						rf.print(LOG_ALL, "appendentries unacceptable")
						rf.appendEntriesHandleFinished <- true
						continue
					}
					rf.print(LOG_ALL, "收到心跳包,重置选举计时器")
					rf.electionTimer.start()

					request.reply.Term = rf.currentTerm
					rf.appendEntriesHandleFinished <- true
					DPrintf("心跳包请求处理完毕")
				//收到投票
				case request := <-rf.receiveRequestVote:
					rf.print(LOG_ALL, "收到投票请求")
					acceptable := rf.voteCommonRequestHandler(request)
					if !acceptable {
						rf.print(LOG_ALL, "unacceptable request vote")
						rf.requestVoteHandleFinished <- true
						continue
					}
					rf.electionTimer.start()

					request.reply.Term = rf.currentTerm

					//&& rf.isNewestLog(args.LastLogIndex, args.LastLogTerm ) //选举限制 5.2 5.4
					rf.print(LOG_ALL, "votefor:%v", rf.voteFor)
					if rf.voteFor == -1 || rf.voteFor == request.args.CandidateId {
						rf.print(LOG_ALL, "向xx投票")
						request.reply.VoteGranted = true
						rf.voteFor = request.args.CandidateId
					}
					rf.requestVoteHandleFinished <- true
				}
			}
		}
	}()

	//并发发送网络请求线程 leader发送心跳包 , candidate请求投票 , follower不发送任何请求
	go func() {
		for {
			select {
			case <-rf.concurrentSendAppendEntries:
				rf.print(LOG_ALL, "群发心跳包")
				///////////////////////////////////////////////
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
				///////////////////////////////////////////////
			case <-rf.concurrentSendVote:
				rf.print(LOG_ALL, "群发投票")
				///////////////////////////////////
				for i, _ := range rf.peers {
					if i == rf.me {
						continue
					}
					go func(i int) {
						args := &RequestVoteArgs{
							Term:        rf.currentTerm,
							CandidateId: rf.me,
						}
						reply := &RequestVoteReply{}
						rf.sendRequestVote(i, args, reply)
					}(i)
				}
				///////////////////////////////////
			}
		}
	}()

	//选举超时定时器线程
	go func() {
		ms := 5

		for {
			time.Sleep(time.Duration(ms) * time.Millisecond)
			rf.electionTimer.tick(ms)
			if rf.electionTimer.reachTimeOut() {
				DPrintf("选举计时器超时")
				rf.startNewElection <- true
				DPrintf("触发新选举事件")
			}
		}
	}()

	//心跳超时定时器
	go func() {
		ms := 5

		for {
			if rf.role != ROLE_LEADER {
				continue
			}
			time.Sleep(time.Duration(ms) * time.Millisecond)
			rf.heartBeatTimer.tick(ms)
			if rf.heartBeatTimer.reachTimeOut() {
				rf.concurrentSendAppendEntries <- true
				//restart heartBeatTimer
				rf.heartBeatTimer.start()
			}
		}
	}()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}

func (rf *Raft) electionTimeOut() int {
	return int(rand.Int63())%500 + 200
	//return time.Duration((rand.Int63())%500+200) * time.Millisecond
}

func (rf *Raft) appendEntriesCommonHandler(request AppendEntriesRequest) bool {
	args := request.args
	reply := request.reply

	//过期clock , 拒绝请求 , 并告知对方term
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		return false
	}

	//需要更新自己的term , 如果不是follower需要回退到follower
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.becomeFollower(args.Term)
		//可以继续处理该请求
		return true
	}

	return true
}

func (rf *Raft) voteCommonRequestHandler(request VoteRequest) bool {
	args := request.args
	reply := request.reply

	//过期clock , 拒绝请求 , 并告知对方term
	if args.Term < rf.currentTerm {
		reply.Term = rf.currentTerm
		return false
	}

	//需要更新自己的term , 如果不是follower需要回退到follower
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		rf.becomeFollower(args.Term)
		//可以继续处理该请求
		return true
	}

	return true
}

func (rf *Raft) voteCommonResponseHandler(request VoteRequest) bool {
	replys := request.reply
	args := request.args

	//过期clock , 拒绝请求 , 并告知对方term
	if replys.Term < rf.currentTerm {
		args.Term = rf.currentTerm
		return false
	}

	//需要更新自己的term , 如果不是follower需要回退到follower
	if replys.Term > rf.currentTerm {
		rf.currentTerm = replys.Term
		rf.becomeFollower(replys.Term)
		return true
	}

	return true
}

func (rf *Raft) becomeFollower(term int) {

	rf.role = ROLE_FOLLOWER
	rf.voteFor = -1
	rf.currentTerm = term
	rf.persist()
	rf.voteCount = 0
	//rf.print(LOG_ALL, "变成 follower 角色:%v", rf.role)
}

func (rf *Raft) becomeCandidate() {
	if rf.role == ROLE_CANDIDATE {
		rf.print(LOG_ALL, "candidate新一轮选举")
	} else {
		rf.print(LOG_ALL, "变成 candidate")
	}

	rf.role = ROLE_CANDIDATE
	rf.currentTerm++
	rf.voteFor = rf.me
	//rf.persist()
	rf.voteCount = 1

	rf.print(LOG_VOTE, "开始选举,任期:%v", rf.currentTerm)

	//群发投票请求
	rf.concurrentSendVote <- true
}

func (rf *Raft) becomeLeader() {
	rf.print(LOG_ALL, "变成 leader")
	rf.role = ROLE_LEADER
	rf.voteFor = -1
	rf.persist()
	rf.voteCount = 0

	//复制阶段初始化
	rf.matchIndex = make([]int, rf.peerCount)
	rf.nextIndex = make([]int, rf.peerCount)
	//for i, _ := range rf.nextIndex {
	//rf.nextIndex[i] = rf.lastLogIndex() + 1
	//}

	//开启心跳包定时器线程
	rf.heartBeatTimer.start()

}

/*
 * Timer
 * 模仿TCP协议里的重传Timer
 */

type Timer struct {
	timeoutMs          int
	timePassMs         int
	stopped            bool
	timeoutMsGenerator func() int
}

func (t *Timer) tick(msSinceLastTick int) {
	if t.stopped {
		//DPrintf("timer停止tick")
		return
	}
	//DPrintf("timer tick")
	t.timePassMs += msSinceLastTick
}

func (t *Timer) start() {
	//DPrintf("timer开始")
	t.stopped = false
	t.timeoutMs = t.timeoutMsGenerator()
	t.timePassMs = 0
}

func (t *Timer) reachTimeOut() bool {
	if t.timePassMs > t.timeoutMs {
		//DPrintf("timer超时")
		t.timePassMs = 0
		t.stop()
		return true
	}
	return false
}

func (t *Timer) stop() {
	t.stopped = true
}

func (t *Timer) reset() {
	t.timePassMs = 0
}

// ----------------------- stable ---------------------------- //

// return currentTerm and whether this server
// believes it is the leader.
func (rf *Raft) GetState() (int, bool) {
	rf.print(LOG_ALL, "is leader %v", rf.role == ROLE_LEADER)
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
		//rf.print(LOG_REPLICA_1, "客户端发起command: isLeader:%v index:%v,term:%v", isLeader, index, term)
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
