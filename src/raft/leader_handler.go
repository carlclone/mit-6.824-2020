package raft

//收到投票 , 公共处理
func (rf *Raft) leaderReqsRVHandler(request VoteRequest) {
	rf.finishReqsRVHandle <- true
}

// leader收到心跳 , 公共处理
func (rf *Raft) leaderReqsAEHandler(request AppendEntriesRequest) {
	rf.finishReqsAEHandle <- true
}

//收到心跳响应,公共处理
func (rf *Raft) leaderRespAEHandler(request AppendEntriesRequest) {
	rf.print(LOG_ALL, "leader 收到心跳响应")

	reply := request.reply
	server := reply.From

	if reply.NeedMaintainIndex {
		//rf.print(LOG_REPLICA_1, "维护 nextIndex 和 MatchIndex server:%v nextIndex %v matchIndex %v", server, reply.NextIndex, reply.MatchIndex)

		//2C优化
		// leader要做的事 :   从后往前找到term=conflictTerm的log , 如果找到了 , 要设置nextIndex = 该log的index
		//如果没找到 , nextIndex=conflictIndex
		if reply.ConflictTerm != -1 {
			if reply.ConflictIndex == -1 {
				reply.ConflictIndex = rf.findFirstIndexOfTerm(reply.ConflictTerm)
			}

			//logLen := len(rf.log)
			rf.print(LOG_UN8, "conflict, conflictTerm %v ,conflictIndex %v", reply.ConflictTerm, reply.ConflictIndex)

			rf.nextIndex[server] = reply.ConflictIndex
			rf.print(LOG_UN8, "维护fast backup nextIndex %v", rf.nextIndex[server])

		} else {
			rf.nextIndex[server] = reply.NextIndex
			rf.print(LOG_UN8, "一个一个退 %v", rf.nextIndex[server])
		}

		if reply.MatchIndex != -1 {
			rf.print(LOG_ALL, "维护matchIndex from %v,matchIndex:%v", server, reply.MatchIndex)
			rf.matchIndex[server] = reply.MatchIndex
			rf.updateLeaderCommitStatus()
		}
	}

	return
}
