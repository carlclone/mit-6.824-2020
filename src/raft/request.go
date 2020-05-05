package raft

//发送请求
func (rf *Raft) sendAppendEntries(server int, args *AppendEntriesArgs, reply *AppendEntriesReply) bool {
	if rf.role != ROLE_LEADER {
		return false
	}
	rf.print(LOG_HEARTBEAT, "发送心跳包前给%v 当前角色:%v", server, rf.role)
	args.LeaderCommitIndex = rf.commitIndex
	args.Entries = rf.serverNextEntriesToReplica(server)

	if len(args.Entries) == 0 {
		lastLog := rf.log[rf.lastLogIndex()]
		args.PrevLogIndex = lastLog.Index
		args.PrevLogTerm = lastLog.Term
	} else {
		prevIndex := args.Entries[0].Index - 1
		args.PrevLogIndex = rf.log[prevIndex].Index
		args.PrevLogTerm = rf.log[prevIndex].Term
	}

	rf.print(LOG_HEARTBEAT, "发送心跳包给%v 当前角色:%v", server, rf.role)

	ok := rf.peers[server].Call("Raft.AppendEntries", args, reply)

	if rf.othersHasBiggerTerm(reply.Term, rf.currentTerm) {
		rf.becomeFollower(reply.Term)
		return ok
	}

	if reply.NeedMaintainIndex {
		rf.print(LOG_REPLICA_1, "维护 nextIndex 和 MatchIndex server:%v reply%v", server, reply)
		rf.nextIndex[server] = reply.NextIndex
		if reply.MatchIndex != -1 {
			rf.matchIndex[server] = reply.MatchIndex
		}
	}

	return ok
}

// 发送请求
func (rf *Raft) sendRequestVote(server int, args *RequestVoteArgs, reply *RequestVoteReply) bool {

	rf.print(LOG_VOTE, "发送 RV")
	ok := rf.peers[server].Call("Raft.RequestVote", args, reply)

	if rf.othersHasBiggerTerm(reply.Term, rf.currentTerm) {
		rf.becomeFollower(reply.Term)
		return ok
	}

	if ok {
		if reply.VoteGranted {
			rf.mu.Lock()
			rf.voteCount++
			rf.mu.Unlock()
			rf.print(LOG_VOTE, "获得选票数:%v", rf.voteCount)
		}
	}

	if rf.voteCount > len(rf.peers)/2 {
		rf.becomeLeader()
	}

	return ok
}
