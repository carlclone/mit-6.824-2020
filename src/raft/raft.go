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
	mu2       sync.Mutex          // Lock to protect shared access to this peer's state
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

	resetTimer chan bool
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

	//channels
	rf.initChannels()

	// initialize from state persisted before a crash
	rf.readPersist(persister.ReadRaftState())

	rf.print(LOG_ALL, "创建了%v", rf.me)

	/*
	 * 可以并发 : 群发投票请求 , 群发心跳包
	 * 不可以并发: 投票响应处理,心跳响应处理,投票请求处理,心跳请求处理,定时器事件 , Start()
	 */

	/*
	 * 事件流向 : 定时器线程,请求,响应产生事件,  主事件线程接收 ,处理 , 如果有需要并发的(网络请求),分配并发事件到并发线程
	 */

	/*
	 * Timer 线程
	 */
	go func() {
		for {
			select {
			case <-time.After(time.Duration((rand.Int63())%300+150) * time.Millisecond):
				switch rf.role {
				case ROLE_LEADER:
				case ROLE_CANDIDATE:
					rf.candElectTimeoutHandler()
				case ROLE_FOLLOWER:
					rf.followerElectTimeoutHandler()
				}
			case <-rf.resetTimer:
			}
		}
	}()

	//主事件循环线程
	go func() {
		for {
			select {
			case request := <-rf.reqsAERcvd:
				switch rf.role {
				case ROLE_LEADER:
					rf.leaderReqsAEHandler(request)
				case ROLE_CANDIDATE:
					rf.print(LOG_ALL, "CC1")
					rf.candReqsAEHandler(request)
				case ROLE_FOLLOWER:
					rf.resetTimer <- true
					rf.followerReqsAEHandler(request)
				}

			case request := <-rf.reqsRVRcvd:
				rf.print(LOG_ALL, "收到投票请求")
				switch rf.role {
				case ROLE_LEADER:
					rf.leaderReqsRVHandler(request)
				case ROLE_CANDIDATE:
					rf.print(LOG_ALL, "CC2")
					rf.candReqsRVHandler(request)
				case ROLE_FOLLOWER:
					rf.print(LOG_ALL, "follower 开始处理投票请求")
					rf.resetTimer <- true
					rf.followerReqsRVHandler(request)
				}
			case request := <-rf.respAERcvd:
				switch rf.role {
				case ROLE_LEADER:
					rf.leaderRespAEHandler(request)
				case ROLE_CANDIDATE:
				case ROLE_FOLLOWER:
				}
			case request := <-rf.respRVRcvd:
				rf.print(LOG_ALL, "收到投票响应")
				switch rf.role {
				case ROLE_LEADER:
				case ROLE_CANDIDATE:
					rf.print(LOG_ALL, "CC4")
					rf.candRespRVHandler(request)
				case ROLE_FOLLOWER:
				}
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

	//心跳定时器线程
	go func() {
		for {
			if rf.role == ROLE_LEADER {
				rf.concurrentSendAE()
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	return rf
}

// become的同时要重置/初始化角色相关属性 ,channel
func (rf *Raft) becomeFollower(term int) {
	rf.print(LOG_ALL, "becomeFollower")
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

	rf.currentTerm++
	rf.voteFor = rf.me
	rf.persist()
	rf.voteCount = 1

	//rf.initChannels()
	rf.role = ROLE_CANDIDATE
	rf.concurrentSendRV()

	//群发投票请求
	//rf.concurrentSendVote <- true
}

func (rf *Raft) becomeLeader() {
	rf.print(LOG_ALL, "becomeLeader")
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
	rf.role = ROLE_LEADER

	//开启心跳包定时器线程
	//rf.heartBeatTimer.start()

}

func (rf *Raft) initChannels() {
	rf.electTimeOut = make(chan bool, 50)
	rf.someOneVoted = make(chan bool, 50)

	rf.concurrentSendVote = make(chan bool, 50)
	rf.concurrentSendAppendEntries = make(chan bool, 50)

	rf.finishReqsRVHandle = make(chan bool, 50) //需要缓冲 channel , 否则leader会发生阻塞
	rf.finishReqsAEHandle = make(chan bool, 50)

	rf.reqsRVRcvd = make(chan VoteRequest, 50)
	rf.reqsAERcvd = make(chan AppendEntriesRequest, 50)

	rf.respRVRcvd = make(chan VoteRequest, 50)
	rf.respAERcvd = make(chan AppendEntriesRequest, 50)

	rf.resetTimer = make(chan bool, 50)
}
