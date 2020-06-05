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

				reply := &AppendEntriesReply{}
				rf.sendAppendEntries(i, args, reply)
			}(i)

		}
	}
}

func (rf *Raft) concurrentSendRV() {
	rf.print(LOG_ALL, "群发投票")
	for i, _ := range rf.peers {
		if i == rf.me {
			continue
		}
		go func(i int) {
			args := &RequestVoteArgs{
				Term:        rf.currentTerm,
				CandidateId: rf.me,
			}
			reply := &RequestVoteReply{}
			rf.sendRequestVote(i, args, reply)
		}(i)
	}
}
