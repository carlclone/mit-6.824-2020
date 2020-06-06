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
import "labrpc"

// import "bytes"
// import "../labgob"

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

	peerCount      int
	voteCount      int
	electionTimer  Timer
	heartBeatTimer Timer

	reqsRVRcvd chan VoteRequest
	reqsAERcvd chan AppendEntriesRequest

	respRVRcvd chan VoteRequest
	respAERcvd chan AppendEntriesRequest

	finishReqsRVHandle chan bool
	finishReqsAEHandle chan bool

	electTimeOut chan bool

	concurrentSendVote          chan bool
	concurrentSendAppendEntries chan bool

	someOneVoted chan bool
	applyCh      chan ApplyMsg
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
	rf.log = append(rf.log, Entry{}) //设置dummyHead
	rf.applyCh = applyCh

	rf.electionTimer = Timer{stopped: true, timeoutMsGenerator: rf.electionTimeOut}
	rf.heartBeatTimer = Timer{stopped: true, timeoutMsGenerator: func() int {
		return 20
	}}

	//channels
	rf.initChannels()

	/*
	 * 可以并发 : 群发投票请求 , 群发心跳包
	 * 不可以并发: 投票响应处理,心跳响应处理,投票请求处理,心跳请求处理,定时器事件 , Start()
	 */

	/*
	 * 事件流向 : 定时器线程,请求,响应产生事件,  主事件线程接收 ,处理 , 如果有需要并发的(网络请求),分配并发事件到并发线程
	 */
	//主事件循环线程
	go func() {
		for {
			switch rf.role {

			case ROLE_LEADER:
				select {
				//不能在 select 加锁,leader 可能断开了收不到任何请求和响应,但此时可以 start
				//处理心跳请求
				case request := <-rf.reqsAERcvd:
					rf.mu.Lock() //和start互斥
					rf.leaderReqsAEHandler(request)
					rf.mu.Unlock()
				//处理投票请求
				case request := <-rf.reqsRVRcvd:
					rf.mu.Lock() //和start互斥
					rf.leaderReqsRVHandler(request)
					rf.mu.Unlock()
				//处理心跳响应
				case request := <-rf.respAERcvd:
					rf.mu.Lock() //和start互斥
					rf.leaderRespAEHandler(request)
					rf.mu.Unlock()
				}

			case ROLE_CANDIDATE:
				//rf.heartBeatTimer.stop()
				//rf.electionTimer.start()
				select {
				//处理心跳请求
				case request := <-rf.reqsAERcvd:
					rf.candReqsAEHandler(request)
				//处理投票请求
				case request := <-rf.reqsRVRcvd:
					rf.candReqsRVHandler(request)
				//选举超时,新一轮选举
				case <-time.After(time.Duration((rand.Int63())%150+150) * time.Millisecond):
					rf.candElectTimeoutHandler()
				//处理投票响应
				case request := <-rf.respRVRcvd:
					rf.candRespRVHandler(request)
				}

				//rf.electionTimer.stop()

			case ROLE_FOLLOWER:
				//rf.heartBeatTimer.stop()
				//rf.electionTimer.start()

				select {
				//选举超时
				case <-time.After(time.Duration((rand.Int63())%150+150) * time.Millisecond):
					rf.followerElectTimeoutHandler()
				//收到心跳包
				case request := <-rf.reqsAERcvd:
					rf.followerReqsAEHandler(request)
				//收到投票
				case request := <-rf.reqsRVRcvd:
					rf.followerReqsRVHandler(request)
				}
				//rf.electionTimer.stop()
			}
		}
	}()

	//并发发送网络请求线程 leader发送心跳包 , candidate请求投票 , follower不发送任何请求
	//go func() {
	//	for {
	//		select {
	//		case <-rf.concurrentSendAppendEntries:
	//			rf.concurrentSendAE()
	//		case <-rf.concurrentSendVote:
	//			rf.concurrentSendRV()
	//		}
	//		time.Sleep(5 * time.Millisecond)
	//	}
	//}()

	go func() {
		for {
			if rf.role == ROLE_LEADER {
				rf.concurrentSendAE()
			}
			time.Sleep(10 * time.Millisecond)
		}

	}()

	go func() {
		for {
			switch rf.role {
			case ROLE_LEADER:
				rf.updateLeaderCommitStatus()

			}
			rf.tryApply()
			time.Sleep(30 * time.Millisecond)
		}
	}()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	return rf
}

// become的同时要重置/初始化角色相关属性 ,channel
func (rf *Raft) becomeFollower(term int) {
	rf.print(LOG_ALL, "becomeFollower")
	if rf.role == ROLE_LEADER {
		rf.heartBeatTimer.stop()
	}
	rf.role = ROLE_FOLLOWER
	rf.voteFor = -1
	rf.currentTerm = term
	rf.persist()
	rf.voteCount = 0

	//转变成follower的时候不能重置channel , 因为还得继续处理请求
	//rf.initChannels()
	//rf.print(LOG_ALL, "变成 follower 角色:%v", rf.role)
}

func (rf *Raft) becomeCandidate() {
	if rf.role == ROLE_CANDIDATE {
		rf.print(LOG_ALL, "newCandidateRound")
	} else {
		rf.print(LOG_ALL, "becomeCandidate")
	}

	rf.role = ROLE_CANDIDATE
	rf.currentTerm++
	rf.voteFor = rf.me
	//rf.persist()
	rf.voteCount = 1

	rf.initChannels()

	//群发投票请求
	//rf.concurrentSendVote <- true
	rf.concurrentSendRV()
}

func (rf *Raft) becomeLeader() {
	rf.print(LOG_ALL, "becomeLeader")
	rf.role = ROLE_LEADER
	rf.voteFor = -1
	rf.persist()
	rf.voteCount = 0

	//复制阶段初始化
	rf.matchIndex = make([]int, rf.peerCount)
	rf.nextIndex = make([]int, rf.peerCount)
	for i, _ := range rf.nextIndex {
		rf.nextIndex[i] = rf.lastLogIndex() + 1
	}

	rf.initChannels()

	//开启心跳包定时器线程
	//rf.heartBeatTimer.start()

}

func (rf *Raft) initChannels() {
	rf.electTimeOut = make(chan bool, 50)
	rf.someOneVoted = make(chan bool, 50)

	rf.concurrentSendVote = make(chan bool, 50)
	rf.concurrentSendAppendEntries = make(chan bool, 50)

	rf.finishReqsRVHandle = make(chan bool)
	rf.finishReqsAEHandle = make(chan bool)

	rf.reqsRVRcvd = make(chan VoteRequest, 50)
	rf.reqsAERcvd = make(chan AppendEntriesRequest, 50)

	rf.respRVRcvd = make(chan VoteRequest, 50)
	rf.respAERcvd = make(chan AppendEntriesRequest, 50)
}
