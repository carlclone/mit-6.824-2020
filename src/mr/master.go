package mr

import (
	"log"
	"sync"
	"time"
)
import "net"
import "os"
import "net/rpc"
import "net/http"

type Master struct {
	// Your definitions here.
	MapExecuting    map[string]*Task // 文件名->task
	ReduceExecuting map[string]*Task

	MapUnExecute    []*Task
	ReduceUnExecute []*Task

	MapExecuted    map[string]*Task
	ReduceExecuted map[string]*Task

	mu sync.Mutex
}

// Your code here -- RPC handlers for the worker to call.
func (m *Master) RetrieveTask(args *AskForTaskArgs, reply *AskForTaskReply) error {
	//加锁
	m.mu.Lock()
	defer m.mu.Unlock()

	//取出一个任务
	task := m.MapUnExecute[0]
	m.MapUnExecute = m.MapUnExecute[1:]
	//放入执行中
	m.MapExecuting[task.FileName] = task

	//返回给客户端
	reply.Task = task

	return nil
}

func (m *Master) UpdateTask(args *UpdateTaskStatusArgs, reply *UpdateTaskStatusReply) error {
	if task, ok := m.MapExecuting[args.Task.FileName]; !ok {
		return nil
	}

	task.St

}

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (m *Master) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

//
// start a thread that listens for RPCs from worker.go
//
func (m *Master) server() {
	rpc.Register(m)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := masterSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrmaster.go calls Done() periodically to find out
// if the entire job has finished.
//
func (m *Master) Done() bool {
	ret := false

	// Your code here.

	return ret
}

//
// create a Master.
// main/mrmaster.go calls this function.
// nReduce is the number of reduce tasks to use.  把中间值哈希成10份
//
func MakeMaster(files []string, nReduce int) *Master {
	m := Master{}

	//初始化未执行Task数组
	m.MapUnExecute = []*Task{}
	for _, file := range files {
		m.MapUnExecute = append(m.MapUnExecute, &Task{
			Type:         TYPE_MAP,
			FileName:     file,
			Status:       WAIT_FOR_EXECUTE,
			RetrieveTime: time.Now(),
		})
	}

	//初始化 map
	m.MapExecuted = make(map[string]*Task)
	m.MapExecuting = make(map[string]*Task)

	m.server()
	return &m
}
