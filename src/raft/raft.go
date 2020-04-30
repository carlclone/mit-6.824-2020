package raft

//
// this is an outline of the API that raft must expose to
// the service (or tester). see comments below for
// each of these functions for more details.
//
// rf = Make(...)
//   create a new Raft server.
// rf.Start(Command interface{}) (index, Term, isleader)
//   start agreement on a new log entry
// rf.GetState() (Term, isLeader)
//   ask a Raft for its current Term, and whether it thinks it is leader
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
	log         []Entry

	//volatile
	commitIndex int //log里最大的已commited的index
	lastApplied int //log里最大的index , (applied的)

	//volatile / only leader
	nextIndex  []int
	matchIndex []int

	//other
	role                 int
	voteCount            int
	receiveAppendEntries chan bool
	receiveVoteReqs      chan bool
}

type Entry struct {
	Command interface{}
	//任期号用来检测多个日志副本之间的不一致情况
	//Leader 在特定的任期号内的一个日志索引处最多创建一个日志条目，同时日志条目在日志中的位置也从来不会改变。该点保证了上面的第一条特性。
	Term int
}

//请求结构
type AppendEntriesArgs struct {
	Term              int //当前 Term
	LeaderId          int
	PrevLogIndex      int //上一个log的index
	PrevLogTerm       int //上一个log的term
	Entries           []Entry
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

	DPrintf("开始处理心跳请求 %v %v %v", rf.me, args, reply)
	reply.Success = false
	// 1
	if args.Term < rf.currentTerm {
		DPrintf("term过期请求 %v %v %v", rf.me, args, reply)
		return
	}

	//2  leader 日志中该日志条目之前的所有日志条目也都会被提交 ，包括由其他 leader 创建的条目
	//在发送 AppendEntries RPC 的时候，leader 会将前一个日志条目的索引位置和任期号包含在里面。
	// 如果 follower 在它的日志中找不到包含相同索引位置和任期号的条目，那么他就会拒绝该新的日志条目。
	if len(rf.log)-1 < args.PrevLogIndex {
		DPrintf("AE2")
		return
	}
	//如果 follower 的日志和 leader 的不一致，那么下一次 AppendEntries RPC 中的一致性检查就会失败。
	prevEntry := rf.log[args.PrevLogIndex]
	if prevEntry.Term != args.PrevLogTerm {
		DPrintf("log 不一致")
		return
	}

	//3  follower 中跟 leader 冲突的日志条目会被 leader 的日志条目覆盖
	//AppendEntries RPC 就会成功，将 follower 中跟 leader 冲突的日志条目全部删除然后追加 leader 中的日志条目
	//这步只需要删除
	// delete prevLogIndex+1 ~ logendIndex
	rf.log = rf.log[:args.PrevLogIndex+1]

	//4 append any new entries not already in the log 然后追加 leader 中的日志条目,如果有需要追加的日志条目的话
	//这步是追加(和覆盖)
	//prevLogIndex+1 ~ logendIndex
	for _, entry := range args.Entries {
		rf.log = append(rf.log, entry)
	}

	DPrintf("%v 处理后的日志 %v", rf.me, rf.log)

	//5 follower 更新 commitIndex
	if args.LeaderCommitIndex > rf.commitIndex {
		if args.LeaderCommitIndex > len(rf.log)-1 {
			rf.commitIndex = len(rf.log) - 1
		} else {
			rf.commitIndex = args.LeaderCommitIndex
		}
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

	DPrintf("心跳请求处理完毕 %v %v %v", rf.me, args, reply)
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
	//后半句 :
	//Raft 使用投票的方式来阻止 candidate 赢得选举除非该 candidate 包含了所有已经提交的日志条目
	// Raft 通过比较两份日志中最后一条日志条目的索引值和任期号来定义谁的日志比较新。
	// 如果两份日志最后条目的任期号不同，那么任期号大的日志更新。
	// 如果两份日志最后条目的任期号相同，那么日志较长的那个更新。
	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) &&
		(args.LastLogTerm > rf.lastLogTerm() || (args.LastLogTerm == rf.lastLogTerm() && args.LastLogIndex >= rf.lastLogIndex())) {
		reply.VoteGranted = true
	}

	DPrintf("%v的投票请求处理完毕  %v %v", args.CandidateId, args, reply)
}

//发送请求
func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	//DPrintf("发出心跳请求 %v %v %v", rf.me, args, reply)
	if rf.lastLogIndex() >= rf.nextIndex[server] {
		DPrintf("nextIndex %v log %v", rf.nextIndex, rf.log)
		args.Entries = rf.log[rf.nextIndex[server]:]
	}

	//继续比对,直到找到第一个一致的元素
	args.PrevLogIndex = rf.nextIndex[server] - 1
	args.Term = rf.currentTerm
	args.PrevLogTerm = rf.log[args.PrevLogIndex].Term
	args.LeaderCommitIndex = rf.commitIndex
	args.LeaderId = rf.me

	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)

	//在被 follower 拒绝之后，leaer 就会减小 nextIndex 值并重试 AppendEntries RPC
	if !reply.Success {
		rf.nextIndex[server]--
	}

	//rules for all server (reqs and response)
	if reply.Term > rf.currentTerm {
		DPrintf("%v %v 收到新term", rf.me, rf.role)
		rf.mu.Lock()
		rf.currentTerm = args.Term
		rf.role = ROLE_FOLLOWER
		rf.mu.Unlock()
	}

	if reply.Success {
		rf.nextIndex[server] = rf.lastLogIndex() + 1
		rf.matchIndex[server] = rf.lastLogIndex()
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

		//当选出一个新 leader 时，该 leader 将所有 nextIndex 的值都初始化为自己最后一个日志条目的 index 加1
		rf.nextIndex = make([]int, len(rf.peers))
		for i, _ := range rf.nextIndex {
			rf.nextIndex[i] = len(rf.log) - 1 + 1
		}
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
	rf.log = append(rf.log, Entry{})
	// Your initialization code here (2A, 2B, 2C).

	rf.matchIndex = make([]int, len(rf.peers))

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

				//一旦创建该日志条目的 leader 将它复制到过半的服务器上，该日志条目就会被提交
				//同时，leader 日志中该日志条目之前的所有日志条目也都会被提交 ，包括由其他 leader 创建的条目
				// rules for servers 最后一条
				//if exists an N >commitIndex ,
				N := rf.commitIndex + 1
				// majority of matchIndex[i] >= N , log.Term=currentTerm ,
				majority := 0

				for i, index := range rf.matchIndex {
					if rf.matchIndex[i] >= N && rf.log[index].Term == rf.currentTerm {
						majority++
					}
				}
				if majority >= 2 {
					DPrintf("matchIndex %v", rf.matchIndex)
				}

				//set commitIndex=N
				if majority > len(rf.peers)/2 {
					rf.commitIndex = N
				}

			}
			time.Sleep(20 * time.Millisecond)
		}
	}()

	go func() {
		for {

			//Follower 一旦知道某个日志条目已经被提交就会将该日志条目应用到自己的本地状态机中
			// 所有committed 但未 applied 的 Entry
			rf.applyCommitIndexLog(applyCh)
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
// agreement on the next Command to be appended to Raft's log. if this
// server isn't the leader, returns false. otherwise start the
// agreement and return immediately. there is no guarantee that this
// Command will ever be committed to the Raft log, since the leader
// may fail or lose an election. even if the Raft instance has been killed,
// this function should return gracefully.
//
// the first return value is the index that the Command will appear at
// if it's ever committed. the second return value is the current
// Term. the third return value is true if this server believes it is
// the leader.
//
func (rf *Raft) Start(command interface{}) (int, int, bool) {

	// Your code here (2B).
	isLeader := rf.role == ROLE_LEADER
	index := -1
	term := rf.currentTerm

	if isLeader {

		rf.log = append(rf.log, Entry{command, rf.currentTerm})
		index = rf.lastLogIndex()
	}

	DPrintf("%v 客户端请求:%v leader:%v index:%v", rf.me, command, isLeader, index)
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

func (rf *Raft) lastLogIndex() int {
	return len(rf.log) - 1
}

func (rf *Raft) lastLogTerm() int {
	return rf.log[rf.lastLogIndex()].Term
}

func (rf *Raft) applyCommitIndexLog(applyCh chan ApplyMsg) {
	DPrintf("%v commitIndex%v lastApplied%v", rf.me, rf.commitIndex, rf.lastApplied)
	for rf.commitIndex > rf.lastApplied {

		rf.lastApplied++
		DPrintf("%v write to applyCh %v , commitIndex:%v", rf.me, rf.log[rf.lastApplied], rf.commitIndex)
		applyCh <- ApplyMsg{Command: rf.log[rf.lastApplied].Command, CommandIndex: rf.lastApplied, CommandValid: true}
	}
}
