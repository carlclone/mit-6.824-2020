package raft

//处理请求
func (rf *Raft) AppendEntries(args *AppendEntriesArgs, reply *AppendEntriesReply) {
	rf.print(LOG_HEARTBEAT, "收到心跳包来自%v", args.LeaderId)
	rf.mu.Lock()
	defer rf.mu.Unlock()
	reply.Success = false
	reply.NeedMaintainIndex = false
	reply.From = rf.me
	reply.ConflictIndex = -1
	reply.ConflictTerm = -1

	if rf.othersHasSmallTerm(args.Term, rf.currentTerm) {
		reply.Term = rf.currentTerm
		reply.NeedMaintainIndex = true
		reply.NextIndex = rf.lastLogIndex() + 1
		return
	}
	rf.receiveAppendEntries <- true

	if rf.othersHasBiggerTerm(args.Term, rf.currentTerm) {
		rf.becomeFollower(args.Term)
		reply.Term = rf.currentTerm
		//return
	}

	if rf.role == ROLE_CANDIDATE {
		rf.becomeFollower(args.Term)
	}

	success := rf.comparePrevLog(args.PrevLogTerm, args.PrevLogIndex)

	if success {
		if len(args.Entries) != 0 {
			rf.appendLeadersLog(args.Entries)
			reply.Success = true
			reply.MatchIndex = args.PrevLogIndex + len(args.Entries)
			reply.NextIndex = rf.lastLogIndex() + 1
			reply.NeedMaintainIndex = true
			rf.print(LOG_REPLICA_1, "成功处理心跳包")

		}
		rf.updateFollowerCommitIndex(args.LeaderCommitIndex)
	} else {
		//2C的优化实现
		if args.PrevLogIndex <= len(rf.log)-1 {
			prevlog := rf.log[args.PrevLogIndex]
			reply.ConflictTerm = prevlog.Term
			reply.ConflictIndex = rf.findFirstIndexOfTerm(prevlog.Term)
		}
		reply.NextIndex = args.PrevLogIndex
		reply.MatchIndex = -1
		reply.Success = true
		reply.NeedMaintainIndex = true

	}

	reply.Term = rf.currentTerm
	return
}

func (rf *Raft) RequestVote(args *RequestVoteArgs, reply *RequestVoteReply) {
	rf.mu.Lock()
	defer rf.mu.Unlock()
	rf.print(LOG_VOTE, "收到投票请求 %v", args.CandidateId)

	if rf.othersHasSmallTerm(args.Term, rf.currentTerm) {
		reply.Term = rf.currentTerm
		return
	}
	rf.receiveVoteReqs <- true

	if rf.othersHasBiggerTerm(args.Term, rf.currentTerm) {
		rf.becomeFollower(args.Term)
	}

	if rf.role != ROLE_FOLLOWER {
		return
	}

	//2 前半句
	if (rf.votedFor == -1 || rf.votedFor == args.CandidateId) && rf.isNewestLog(args.LastLogIndex, args.LastLogTerm) { //选举限制 5.2 5.4
		reply.VoteGranted = true
		rf.votedFor = args.CandidateId
	}

	reply.Term = rf.currentTerm

}
