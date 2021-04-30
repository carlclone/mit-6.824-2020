package raft

import (
	"bytes"
	"labgob"
)

// Raft状态持久化
// save Raft's persistent state to stable storage,
// where it can later be retrieved after a crash and restart.
// see paper's Figure 2 for a description of what should be persistent.
//
func (rf *Raft) persist() {
	data := rf.getRaftPersistState()
	rf.persister.SaveRaftState(data)
}

func (rf *Raft) getRaftPersistState() []byte {
	w := new(bytes.Buffer)
	e := labgob.NewEncoder(w)
	e.Encode(rf.voteFor)
	e.Encode(rf.currentTerm)
	e.Encode(rf.log)
	data := w.Bytes()
	return data
}

//快照相关
func (rf *Raft) OutOfBound(limitBytes int) bool {
	return rf.persister.RaftStateSize() > limitBytes
}

func (rf *Raft) SaveSnapShot(shot []byte, index int) {
	rf.persister.SaveStateAndSnapshot(rf.getRaftPersistState(), shot)

	//discard log entrys
}

type InstallSnapShotArgs struct {
	Term              int     //当前 term
	LeaderId          int     // leader的id
	PrevLogIndex      int     // 前一个log , 用来比对不一致的情况
	PrevLogTerm       int     // 同上
	Entries           []Entry //leader "猜测" follower缺失的entry
	LeaderCommitIndex int     // leader已提交的最后一个index
}

type InstallSnapShotReply struct {
	Term              int
	Success           bool //
	NextIndex         int  //follower告知leader下一个要复制的index
	MatchIndex        int  //follower告知leader 维护matchIndex
	AppendSuccess     bool //是否append成功
	NeedMaintainIndex bool //是否需要维护nextIndex
	From              int  //reply来自?

	ConflictTerm  int
	ConflictIndex int
}

// if followCommitIndex < leaderOldestIndex
//sendInstallRPC(ss,index)  //保证exactly once , 怎么做
func (rf *Raft) sendInstallSnapShot(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	rf.print(LOG_ALL, "sendAe_ ")
	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)
	rf.print(LOG_ALL, "sendAe OK:%v", ok)
	if ok {
		rf.respAERcvd <- AppendEntriesRequest{args, reply}
	}
	return ok
}

//event: receved InstallSSRPC
//discardFrom(index)
//update commitIndex and lastApplied
//applyCh <- InstallSSCommand
//persistSS()
//persistRaftState()
//
//https://raft.github.io/
//https://www.cnblogs.com/linbingdong/p/6442673.html
func (rf *Raft) InstallSnapShot(args *AppendEntriesArgs, reply *AppendEntriesReply) {

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
