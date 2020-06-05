package raft

func (rf *Raft) candElectTimeoutHandler() {
	rf.becomeCandidate()
}

func (rf *Raft) handleRespRV(request VoteRequest) {
	args := request.args
	reply := request.reply

	rf.print(LOG_ALL, "收到的投票结果 %v", reply.VoteGranted)
	acceptable := rf.voteCommonResponseHandler(VoteRequest{args, reply})
	if !acceptable {
		rf.print(LOG_ALL, "unacceptable")
		return
	}

	if rf.role != ROLE_CANDIDATE {
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

func (rf *Raft) candRespRVHandler(request VoteRequest) {

}

func (rf *Raft) candReqsRVHandler(request VoteRequest) {
	acceptable := rf.voteCommonRequestHandler(request)
	if !acceptable {
		rf.print(LOG_ALL, "unacceptable request vote")
		rf.finishReqsRVHandle <- true
		return
	}
}

func (rf *Raft) candReqsAEHandler(request AppendEntriesRequest) {
	rf.becomeFollower(request.args.Term)
	rf.finishReqsAEHandle <- true
}
