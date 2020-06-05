package raft

func (rf *Raft) concurrentSendAE() {

	rf.print(LOG_ALL, "群发心跳包")
	for i, _ := range rf.peers {
		if i != rf.me {
			go func(i int) {

				args := &AppendEntriesArgs{
					Term:     rf.currentTerm,
					LeaderId: rf.me,
				}

				args.LeaderCommitIndex = rf.commitIndex
				args.Entries = rf.serverNextEntriesToReplica(i)

				if len(args.Entries) == 0 {
					lastLog := rf.log[rf.lastLogIndex()]
					args.PrevLogIndex = lastLog.Index
					args.PrevLogTerm = lastLog.Term
				} else {
					prevIndex := args.Entries[0].Index - 1
					args.PrevLogIndex = rf.log[prevIndex].Index
					args.PrevLogTerm = rf.log[prevIndex].Term
				}
				reply := &AppendEntriesReply{}
				rf.sendAppendEntries(i, args, reply)
			}(i)

		}
	}
}

func (rf *Raft) concurrentSendRV() {
	rf.print(LOG_ALL, "群发投票")

	args := &RequestVoteArgs{
		Term:        rf.currentTerm,
		CandidateId: rf.me,
	}
	lastLog := rf.lastLog()
	args.LastLogTerm = lastLog.Term
	args.LastLogIndex = lastLog.Index
	for i, _ := range rf.peers {
		if i == rf.me {
			continue
		}
		go func(i int) {

			reply := &RequestVoteReply{}
			rf.sendRequestVote(i, args, reply)
		}(i)
	}
}

func (rf *Raft) serverNextEntriesToReplica(server int) []Entry {
	nextIndex := rf.nextIndex[server]
	rf.print(LOG_ALL, "nextIndex:::%v", nextIndex)
	var res []Entry
	if rf.lastLogIndex() >= nextIndex {
		res = rf.log[nextIndex:]
	} else {
		res = []Entry{}
	}
	if len(res) != 0 {
		rf.print(LOG_REPLICA_1, "准备复制给 server %v 的 nI%v mI%v res:%v", server, rf.nextIndex[server], rf.matchIndex[server], res)
	}

	return res
}
