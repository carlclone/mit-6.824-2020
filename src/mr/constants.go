package mr

import "time"

const TYPE_MAP = 1
const TYPE_REDUCE = 2

const EXECUTING = 1
const EXECUTED = 2
const WAIT_FOR_EXECUTE = 3

type Task struct {
	Type         int
	FileName     string
	Status       int
	RetrieveTime time.Time
	FinishedTime time.Time
}

func (t *Task) isTimeOut() bool {
	return t.RetrieveTime.Add(time.Duration(10000) * time.Millisecond).After(time.Now())
}
