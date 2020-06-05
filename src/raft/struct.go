package raft

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
	Command      interface{} //被复制的内容
	CommandIndex int         //内容在复制机里的索引
}

const (
	ROLE_LEADER    = 1
	ROLE_CANDIDATE = 2
	ROLE_FOLLOWER  = 3
)

type Entry struct {
	Command interface{}

	/*
	 * 任期号用来检测多个日志副本之间的不一致情况
	 * Leader 在特定的任期号内的一个日志索引处最多创建一个日志条目，同时日志条目在日志中的位置也从来不会改变。该点保证了上面的第一条特性。
	 */

	Term  int //该entry被复制时的term
	Index int //所处位置
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

type RequestVoteArgs struct {
	Term         int //发出请求时的term
	CandidateId  int
	LastLogIndex int //最后一个log的index
	LastLogTerm  int // 同上 , 两个组合起来用于判断该peer是否有最新的log , 没有则不投票
}

type RequestVoteReply struct {
	Term        int
	VoteGranted bool
}

// prev last next , 分别是前一个 最后一个 , 下一个
type AppendEntriesArgs struct {
	Term              int     //当前 term
	LeaderId          int     // leader的id
	PrevLogIndex      int     // 前一个log , 用来比对不一致的情况
	PrevLogTerm       int     // 同上
	Entries           []Entry //leader "猜测" follower缺失的entry
	LeaderCommitIndex int     // leader已提交的最后一个index
}

type AppendEntriesReply struct {
	Term              int
	Success           bool
	NextIndex         int
	MatchIndex        int
	AppendSuccess     bool
	NeedMaintainIndex bool
	From              int

	ConflictTerm  int
	ConflictIndex int
}
