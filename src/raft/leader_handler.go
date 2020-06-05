package raft

//收到投票 , 公共处理
func (rf *Raft) leaderReqsRVHandler(request VoteRequest) {
	acceptable := rf.voteCommonRequestHandler(request)
	if !acceptable {
		rf.print(LOG_ALL, "unacceptable request vote")
		rf.finishReqsRVHandle <- true
		return
	}
}

//收到心跳响应,公共处理
func (rf *Raft) leaderRespAEHandler(request AppendEntriesRequest) {
	args := request.args
	reply := request.reply

	acceptable := rf.aeCommonResponseHandler(AppendEntriesRequest{args, reply})
	if !acceptable {
		rf.print(LOG_ALL, "unacceptable")
		return
	}
}

// leader收到心跳 , 公共处理
func (rf *Raft) leaderReqsAEHandler(request AppendEntriesRequest) {
	//公共处理,并判断是否继续处理该请求
	acceptable := rf.appendEntriesCommonReqsHandler(request)
	if !acceptable {
		rf.print(LOG_ALL, "appendentries unacceptable")
		rf.finishReqsAEHandle <- true
		return
	}
	rf.finishReqsAEHandle <- true
}
