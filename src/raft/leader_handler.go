package raft

func (rf *Raft) leaderReqsRVHandler(request VoteRequest) {
	acceptable := rf.voteCommonRequestHandler(request)
	if !acceptable {
		rf.print(LOG_ALL, "unacceptable request vote")
		rf.finishReqsRVHandle <- true
		return
	}
}

func (rf *Raft) leaderRespAEHandler(request AppendEntriesRequest) {

}

func (rf *Raft) leaderReqsAEHandler(request AppendEntriesRequest) {
	//公共处理,并判断是否继续处理该请求
	acceptable := rf.appendEntriesCommonHandler(request)
	if !acceptable {
		rf.print(LOG_ALL, "appendentries unacceptable")
		rf.finishReqsAEHandle <- true
		return
	}
	rf.finishReqsAEHandle <- true
}
