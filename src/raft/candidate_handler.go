package raft

func (rf *Raft) candElectTimeoutHandler() {
	rf.becomeCandidate()
}

// cand 收到响应投票,票数++ , 判断是否 -> leader
func (rf *Raft) candRespRVHandler(request VoteRequest) {
	args := request.args
	reply := request.reply

	rf.print(LOG_ALL, "收到的投票结果 %v", reply.VoteGranted)
	acceptable := rf.voteCommonResponseHandler(VoteRequest{args, reply})
	if !acceptable {
		rf.print(LOG_ALL, "unacceptable")
		return
	}

	if reply.VoteGranted {
		rf.print(LOG_ALL, "收到支持投票")
		rf.print(LOG_ALL, "vote++")
		rf.voteCount++
		if rf.voteCount > rf.peerCount/2 {
			rf.print(LOG_ALL, "成为leader")
			rf.becomeLeader()
		}
	}
}

//cand收到投票 , 公共处理
func (rf *Raft) candReqsRVHandler(request VoteRequest) {
	acceptable := rf.voteCommonRequestHandler(request)
	if !acceptable {
		rf.print(LOG_ALL, "unacceptable request vote")
		rf.finishReqsRVHandle <- true
		return
	}
}

//cand收到心跳 , 只需要按照公共处理
func (rf *Raft) candReqsAEHandler(request AppendEntriesRequest) {
	//公共处理,并判断是否继续处理该请求
	acceptable := rf.appendEntriesCommonReqsHandler(request)
	if !acceptable {
		rf.print(LOG_ALL, "appendentries unacceptable")
		rf.finishReqsAEHandle <- true
		return
	}

	rf.finishReqsAEHandle <- true
}
