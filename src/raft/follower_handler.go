package raft

func (rf *Raft) followerElectTimeoutHandler() {
	rf.print(LOG_ALL, "follower 选举超时")
	//to candidate
	rf.becomeCandidate()
}

func (rf *Raft) followerReqsAEHandler(request AppendEntriesRequest) {
	rf.print(LOG_ALL, "follower 开始处理心跳")

	reply := request.reply
	args := request.args

	reply.Success = false
	reply.NeedMaintainIndex = false
	reply.From = rf.me
	reply.ConflictIndex = -1
	reply.ConflictTerm = -1

	success := rf.comparePrevLog(args.PrevLogTerm, args.PrevLogIndex)

	if success {
		if len(args.Entries) != 0 {
			rf.print(LOG_ALL, "上一个log一致，有新log，开始append")
			rf.appendLeadersLog(args.Entries)

			reply.Success = true
			reply.MatchIndex = args.PrevLogIndex + len(args.Entries)
			reply.NextIndex = rf.lastLogIndex() + 1
			reply.NeedMaintainIndex = true
		}
		rf.updateFollowerCommitIndex(args.LeaderCommitIndex)
	} else {

		if args.PrevLogIndex <= len(rf.log)-1 {
			//2C的优化实现
			rf.print(LOG_ALL, "2C优化实现")
			prevlog := rf.log[args.PrevLogIndex]
			reply.ConflictTerm = prevlog.Term
			reply.ConflictIndex = rf.findFirstIndexOfTerm(prevlog.Term)
		}
		rf.print(LOG_ALL, "LOG不一致，回退一个")
		reply.NextIndex = args.PrevLogIndex
		reply.MatchIndex = -1
		reply.Success = true
		reply.NeedMaintainIndex = true
	}

	rf.finishReqsAEHandle <- true

}

func (rf *Raft) followerReqsRVHandler(request VoteRequest) {

	rf.print(LOG_ALL, "收到投票请求")

	//follower 判断是否向其投票
	args := request.args
	request.reply.From = rf.me

	if (rf.voteFor == -1 || rf.voteFor == request.args.CandidateId) && rf.isNewestLog(args.LastLogIndex, args.LastLogTerm) {
		rf.print(LOG_ALL, "向%v投票", args.CandidateId)
		request.reply.VoteGranted = true
		rf.voteFor = request.args.CandidateId
		rf.persist()
	}
	rf.finishReqsRVHandle <- true

}

func (rf *Raft) comparePrevLog(prevLogTerm int, prevLogIndex int) bool {
	if prevLogIndex > len(rf.log)-1 {
		return false
	}

	prevlog := rf.log[prevLogIndex]
	var res bool
	if prevlog.Index == prevLogIndex && prevlog.Term == prevLogTerm {
		res = true
	} else {
		res = false
	}
	//rf.print(LOG_REPLICA_1, "比对结果 %v pT:%v pI:%v,ct:%v,ci:%v", res, prevLogTerm, prevLogIndex, prevlog.Term, prevlog.Index)
	return res
}

func (rf *Raft) appendLeadersLog(entries []Entry) {
	//rf.print(LOG_REPLICA_1, "开始 append leader 给的 log,  entry")
	startIndex := entries[0].Index
	//entriesEndIndex := entries[len(entries)-1].Index

	rf.log = rf.log[:startIndex]
	//logEndIndex := len(rf.log) - 1
	rf.log = append(rf.log, entries...)
	//for i := startIndex; i <= entriesEndIndex; i++ {
	//	entry := entries[i-startIndex]
	//	if i <= logEndIndex {
	//		if rf.log[i].Term == entry.Term {
	//			continue
	//		} else {
	//			rf.log[i] = entry
	//		}
	//	} else {
	//		rf.log = append(rf.log, entry)
	//	}
	//}
	rf.persist()
	//rf.print(LOG_REPLICA_1, "append 完毕 %v", rf.log)
}

func (rf *Raft) lastLogIndex() int {
	return len(rf.log) - 1
}

func (rf *Raft) updateFollowerCommitIndex(leaderCommitIndex int) {

	if leaderCommitIndex > rf.commitIndex {
		lastLogIndex := rf.lastLogIndex()
		if leaderCommitIndex > lastLogIndex {
			rf.commitIndex = lastLogIndex
		} else {
			rf.commitIndex = leaderCommitIndex
		}
		rf.print(LOG_PERSIST, "更新commitIndex:%v", rf.commitIndex)
	}

}

func (rf *Raft) lastLog() Entry {
	return rf.log[rf.lastLogIndex()]
}

//Raft 通过比较两份日志中最后一条日志条目的索引值和任期号来定义谁的日志比较新。
// 如果两份日志最后条目的任期号不同，那么任期号大的日志更新。
// 如果两份日志最后条目的任期号相同，那么日志较长的那个更新。
func (rf *Raft) isNewestLog(lastLogIndex int, lastLogTerm int) bool {
	lastLog := rf.lastLog()
	if lastLogTerm > lastLog.Term {
		return true
	}
	if lastLogTerm == lastLog.Term && lastLogIndex >= lastLog.Index {
		return true
	}
	return false
}

func (rf *Raft) findFirstIndexOfTerm(prevLogTerm int) int {
	for _, entry := range rf.log {
		if entry.Term == prevLogTerm {
			return entry.Index
		}
	}
	return -1
}

func (rf *Raft) othersHasBiggerTerm(othersTerm int, currentTerm int) bool {
	if othersTerm > currentTerm {
		//rf.print(LOG_ALL, "收到更大的 term  other%v curr%v", othersTerm, currentTerm)
	}
	return othersTerm > currentTerm
}

func (rf *Raft) othersHasSmallTerm(othersTerm int, term int) bool {
	if othersTerm < term {
		//rf.print(LOG_ALL, "收到过期 term other:%v curr:%v", othersTerm, term)
	}

	return othersTerm < term
}
