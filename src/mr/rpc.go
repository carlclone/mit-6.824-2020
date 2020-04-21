package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"time"
)
import "strconv"

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.
type AskForTaskArgs struct {
}

const (
	ASK_FOR_TASK_FAIL    = 0
	ASK_FOR_TASK_SUCCESS = 1
	ASK_FOR_TASK_DONE    = 2
)

type AskForTaskReply struct {
	Task   *Task
	Status int
}

type TaskFinishedArgs struct {
	Task *Task
	//Status       int
	FinishedTime time.Time
	Intermediate []KeyValue
}

type TaskFinishedReply struct {
}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the master.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func masterSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
