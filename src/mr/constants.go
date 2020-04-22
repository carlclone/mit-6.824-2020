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

	NReduce int
}

func (t *Task) isTimeOut() bool {
	return t.RetrieveTime.Add(10 * time.Second).Before(time.Now())
}

func (t *Task) isTaskExecuted(m *Master) bool {
	table := t.getTaskExecuted(m)

	if _, ok := table[t.FileName]; ok {
		return true
	} else {
		return false
	}
}

func (t *Task) getTaskUnExecutes(m *Master) *[]*Task {
	switch t.Type {
	case TYPE_MAP:
		return &m.MapUnExecute
	case TYPE_REDUCE:
		return &m.ReduceUnExecute
	}
	return nil
}

func (t *Task) getTaskExecuting(m *Master) map[string]*Task {
	switch t.Type {
	case TYPE_MAP:
		return m.MapExecuting
	case TYPE_REDUCE:
		return m.ReduceExecuting
	}
	return nil
}

func (t *Task) getTaskExecuted(m *Master) map[string]*Task {
	switch t.Type {
	case TYPE_MAP:
		return m.MapExecuted
	case TYPE_REDUCE:
		return m.ReduceExecuted
	}
	return nil
}
