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
	return
}
