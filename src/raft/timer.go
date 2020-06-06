package raft

/*
 * Timer
 * 模仿TCP协议里的重传Timer
 */

type Timer struct {
	timeoutMs          int
	timePassMs         int
	stopped            bool
	timeoutMsGenerator func() int
}

func (t *Timer) tick(msSinceLastTick int) {
	if t.stopped {
		//DPrintf("timer停止tick")
		return
	}
	//DPrintf("timer tick")
	t.timePassMs += msSinceLastTick
}

func (t *Timer) start() {
	//DPrintf("timer开始")
	t.stopped = false
	t.timeoutMs = t.timeoutMsGenerator()
	t.timePassMs = 0
}

func (t *Timer) reachTimeOut() bool {
	if t.timePassMs > t.timeoutMs {
		//DPrintf("timer超时")
		t.timePassMs = 0
		t.stop()
		return true
	}
	return false
}

func (t *Timer) stop() {
	t.stopped = true
}

func (t *Timer) reset() {
	t.timePassMs = 0
}

////选举超时定时器线程
//go func() {
//	ms := 5
//	for {
//		time.Sleep(time.Duration(ms) * time.Millisecond)
//		rf.electionTimer.tick(ms)
//		if rf.electionTimer.reachTimeOut() {
//			DPrintf("选举计时器超时")
//			rf.electTimeOut <- true
//			DPrintf("触发新选举事件")
//		}
//	}
//}()
//
////心跳超时定时器
//go func() {
//	ms := 5
//
//	for {
//		if rf.role != ROLE_LEADER {
//			continue
//		}
//		time.Sleep(time.Duration(ms) * time.Millisecond)
//		rf.heartBeatTimer.tick(ms)
//		if rf.heartBeatTimer.reachTimeOut() {
//			rf.concurrentSendAppendEntries <- true
//			//restart heartBeatTimer
//			rf.heartBeatTimer.start()
//		}
//	}
//}()

//rf.electionTimer = Timer{stopped: true, timeoutMsGenerator: rf.electionTimeOut}
//rf.heartBeatTimer = Timer{stopped: true, timeoutMsGenerator: func() int {
//return 20
//}}
