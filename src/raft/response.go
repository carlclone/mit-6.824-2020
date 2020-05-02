package raft

//处理请求
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	reply.Success = false
	reply.NeedMaintainIndex = false
	reply.From = rf.me

	if rf.othersHasSmallTerm(args.Term, rf.currentTerm) {
		reply.Term = rf.currentTerm
		return
	}
	rf.receiveAppendEntries <- true

	if rf.othersHasBiggerTerm(args.Term, rf.currentTerm) {
		rf.becomeFollower(args.Term)
		reply.Term = rf.currentTerm
		return
	}

	success := rf.comparePrevLog(args.PrevLogTerm, args.PrevLogIndex)

	if success {
		if len(args.Entries) != 0 {
			rf.appendLeadersLog(args.Entries)
			reply.Success = true
			reply.NextIndex = rf.lastLogIndex() + 1
			reply.MatchIndex = rf.lastLogIndex()
			reply.NeedMaintainIndex = true
			rf.print(LOG_REPLICA_1, "成功处理心跳包")
		}
	} else {
		reply.NextIndex = rf.lastLogIndex() + 1
		reply.MatchIndex = rf.lastLogIndex()
		reply.Success = true
		reply.NeedMaintainIndex = true
	}

	rf.updateFollowerCommitIndex(args.LeaderCommitIndex)

	reply.Term = rf.currentTerm
	return
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	rf.print(LOG_VOTE, "收到投票请求 %v", args.CandidateId)

	if rf.othersHasSmallTerm(args.Term, rf.currentTerm) {
		reply.Term = rf.currentTerm
		return
	}
	rf.receiveVoteReqs <- true

	if rf.othersHasBiggerTerm(args.Term, rf.currentTerm) {
		rf.becomeFollower(args.Term)
	}

	//2 前半句
	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && rf.isNewestLog(args.LastLogIndex, args.LastLogTerm) { //选举限制 5.2 5.4
		reply.VoteGranted = true
	}

	reply.Term = rf.currentTerm

}
