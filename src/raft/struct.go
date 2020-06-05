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
