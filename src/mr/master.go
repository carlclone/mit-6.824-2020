package mr

import (
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
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

	NReduce int
}

// Your code here -- RPC handlers for the worker to call.
func (m *Master) RetrieveTask(args *AskForTaskArgs, reply *AskForTaskReply) error {
	//加锁
	m.mu.Lock()
	defer m.mu.Unlock()

	//如果map任务都执行完了 , 就分发reduce任务 , 第一次先初始化
	mapFinished := len(m.MapUnExecute) == 0 && len(m.MapExecuting) == 0
	fmt.Println("unexecute:" + strconv.Itoa(len(m.MapUnExecute)) + "\n" + "executing:" + strconv.Itoa(len(m.MapExecuting)))
	if mapFinished && m.ReduceUnExecute == nil {
		//初始化未执行ReduceTask数组
		m.ReduceUnExecute = []*Task{}
		reduceFiles := []string{}
		files, err := ioutil.ReadDir("./mr-mid")
		if err != nil {
			log.Fatal(err)
		}
		for _, f := range files {
			reduceFiles = append(reduceFiles, "mr-mid/"+f.Name())
		}

		for _, file := range reduceFiles {
			m.ReduceUnExecute = append(m.ReduceUnExecute, &Task{
				Type:         TYPE_REDUCE,
				FileName:     file,
				Status:       WAIT_FOR_EXECUTE,
				RetrieveTime: time.Now(),
				NReduce:      m.NReduce,
			})
		}

		//初始化 map
		m.ReduceExecuted = make(map[string]*Task)
		m.ReduceExecuting = make(map[string]*Task)
	}

	if mapFinished {
		//取出reduce一个任务
		task := m.ReduceUnExecute[0]
		m.ReduceUnExecute = m.ReduceUnExecute[1:]
		//放入执行中
		m.ReduceExecuting[task.FileName] = task

		//返回给客户端
		reply.Task = task
		return nil
	}

	//取出一个任务
	task := m.MapUnExecute[0]
	m.MapUnExecute = m.MapUnExecute[1:]
	//放入执行中
	m.MapExecuting[task.FileName] = task

	//返回给客户端
	reply.Task = task

	return nil
}

func (m *Master) UpdateMapTaskFinished(args *TaskFinishedArgs, reply *TaskFinishedReply) error {
	task, ok := m.MapExecuting[args.Task.FileName]
	if !ok {
		return nil
	}

	if task.Type == TYPE_REDUCE {
		task.Status = EXECUTED
		task.FinishedTime = args.Task.FinishedTime

		delete(m.ReduceExecuting, task.FileName)

		m.ReduceExecuted[task.FileName] = task
		return nil
	}

	task.Status = EXECUTED
	task.FinishedTime = args.Task.FinishedTime

	delete(m.MapExecuting, task.FileName)

	m.MapExecuted[task.FileName] = task

	//StoreInterMidiate(args.Intermediate)

	return nil
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

	//初始化未执行MapTask数组
	m.MapUnExecute = []*Task{}
	for _, file := range files {
		m.MapUnExecute = append(m.MapUnExecute, &Task{
			Type:         TYPE_MAP,
			FileName:     file,
			Status:       WAIT_FOR_EXECUTE,
			RetrieveTime: time.Now(),
			NReduce:      nReduce,
		})
	}

	//初始化 map
	m.MapExecuted = make(map[string]*Task)
	m.MapExecuting = make(map[string]*Task)
	m.NReduce = nReduce

	m.server()
	return &m
}
