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

func (rf *Raft) SaveSnapShot(shot []byte, indexToBeDis int) {

	//Raft should save each snapshot with persister.SaveStateAndSnapshot() (don't use files).
	rf.persister.SaveStateAndSnapshot(rf.getRaftPersistState(), shot)
	rf.print(LOG_SNAPSHOT, "rf save snapshot succ")

	//Modify your Raft so that it can be given a log index, discard the entries before that index,

	//and continue operating while storing only log entries after that index.
	//Make sure all the Lab 2 Raft tests still succeed.
	/*
			firstIdx .... disIdx  ....   curIdx
		    2   3 4         5               10

			0   1 2         3

		   rf.log[4:]  ->    [4,curIdx)
	*/
	// actualIdx to be discard
	firstIdx := rf.log[0].Index
	realIdx := indexToBeDis - firstIdx
	rf.log = rf.log[realIdx+1:]

}

func (rf *Raft) ReadSnapshot() []byte {
	return rf.persister.ReadSnapshot()
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
