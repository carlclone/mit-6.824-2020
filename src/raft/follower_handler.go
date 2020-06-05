package raft

func (rf *Raft) followerElectTimeoutHandler() {
	//to candidate
	rf.becomeCandidate()
}

func (rf *Raft) followerReqsAEHandler(request AppendEntriesRequest) {
	rf.finishReqsAEHandle <- true

}

func (rf *Raft) followerReqsRVHandler(request VoteRequest) {

	//follower 判断是否向其投票

	//&& rf.isNewestLog(args.LastLogIndex, args.LastLogTerm ) //选举限制 5.2 5.4
	rf.print(LOG_ALL, "votefor:%v", rf.voteFor)
	if rf.voteFor == -1 || rf.voteFor == request.args.CandidateId {
		rf.print(LOG_ALL, "向xx投票")
		request.reply.VoteGranted = true
		rf.voteFor = request.args.CandidateId
	}
	rf.finishReqsRVHandle <- true

}

func (rf *Raft) othersHasBiggerTerm(othersTerm int, currentTerm int) bool {
	if othersTerm > currentTerm {
		rf.print(LOG_ALL, "收到更大的 term  other%v curr%v", othersTerm, currentTerm)
	}
	return othersTerm > currentTerm
}

func (rf *Raft) othersHasSmallTerm(othersTerm int, term int) bool {
	if othersTerm < term {
		rf.print(LOG_ALL, "收到过期 term other:%v curr:%v", othersTerm, term)
	}

	return othersTerm < term
}
