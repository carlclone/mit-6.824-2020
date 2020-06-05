package raft

func (rf *Raft) voteCommonResponseHandler(request VoteRequest) bool {
	replys := request.reply
	args := request.args

	//过期clock , 拒绝请求 , 并告知对方term
	if replys.Term < rf.currentTerm {
		args.Term = rf.currentTerm
		return false
	}

	//需要更新自己的term , 如果不是follower需要回退到follower
	if replys.Term > rf.currentTerm {
		rf.currentTerm = replys.Term
		rf.becomeFollower(replys.Term)
		return true
	}

	return true
}

func (rf *Raft) aeCommonResponseHandler(request AppendEntriesRequest) bool {
	replys := request.reply
	args := request.args

	//过期clock , 拒绝请求 , 并告知对方term
	if replys.Term < rf.currentTerm {
		rf.print(LOG_ALL, "term过小 %v", replys.Term)
		args.Term = rf.currentTerm
		return false
	}

	//需要更新自己的term , 如果不是follower需要回退到follower
	if replys.Term > rf.currentTerm {
		rf.currentTerm = replys.Term
		rf.becomeFollower(replys.Term)
		return true
	}

	return true
}

func (rf *Raft) appendEntriesCommonReqsHandler(request AppendEntriesRequest) bool {
	args := request.args

	//过期clock , 拒绝请求 , 并告知对方term
	if args.Term < rf.currentTerm {
		request.reply.Term = rf.currentTerm
		return false
	}

	//需要更新自己的term , 如果不是follower需要回退到follower
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		request.reply.Term = rf.currentTerm
		rf.becomeFollower(args.Term)
		//可以继续处理该请求
		return true
	}

	return true
}

func (rf *Raft) voteCommonRequestHandler(request VoteRequest) bool {
	args := request.args
	//reply := request.reply

	//过期clock , 拒绝请求 , 并告知对方term
	if args.Term < rf.currentTerm {
		request.reply.Term = rf.currentTerm
		return false
	}

	//需要更新自己的term , 如果不是follower需要回退到follower
	if args.Term > rf.currentTerm {
		rf.currentTerm = args.Term
		request.reply.Term = rf.currentTerm
		rf.becomeFollower(args.Term)
		//可以继续处理该请求
		return true
	}

	return true
}
