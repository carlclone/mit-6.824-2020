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
