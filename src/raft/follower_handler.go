package raft

func (rf *Raft) followerElectTimeoutHandler() {
	rf.becomeCandidate()
}

func (rf *Raft) followerReqsAEHandler(request AppendEntriesRequest) {
	rf.print(LOG_ALL, "收到心跳包!!!!!!!!!!!!!!")
	//公共处理,并判断是否继续处理该请求
	acceptable := rf.appendEntriesCommonHandler(request)
	if !acceptable {
		rf.print(LOG_ALL, "appendentries unacceptable")
		rf.finishReqsAEHandle <- true
		return
	}
	rf.print(LOG_ALL, "收到心跳包,重置选举计时器")

	request.reply.Term = rf.currentTerm
	rf.finishReqsAEHandle <- true
	DPrintf("心跳包请求处理完毕")
}

func (rf *Raft) followerReqsRVHandler(request VoteRequest) {
	rf.print(LOG_ALL, "收到投票请求")
	acceptable := rf.voteCommonRequestHandler(request)
	if !acceptable {
		rf.print(LOG_ALL, "unacceptable request vote")
		rf.finishReqsRVHandle <- true
		return
	}

	request.reply.Term = rf.currentTerm

	//&& rf.isNewestLog(args.LastLogIndex, args.LastLogTerm ) //选举限制 5.2 5.4
	rf.print(LOG_ALL, "votefor:%v", rf.voteFor)
	if rf.voteFor == -1 || rf.voteFor == request.args.CandidateId {
		rf.print(LOG_ALL, "向xx投票")
		request.reply.VoteGranted = true
		rf.voteFor = request.args.CandidateId
	}
	rf.finishReqsRVHandle <- true

}
