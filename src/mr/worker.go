package mr

import (
	"fmt"
	"time"

	//"io"
	"io/ioutil"
	"os"
	"strconv"
)
import "log"
import "net/rpc"
import "hash/fnv"

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	//var task *Task
	for {
		//获取一个任务
		args := AskForTaskArgs{}
		reply := AskForTaskReply{}
		call("Master.RetrieveTask", &args, &reply)
		task := reply.Task

		filename := task.FileName
		intermediate := []KeyValue{}

		fmt.Println(task.FileName)
		//执行任务 , 参考sequential
		intermediate = exeMapFun(filename, mapf, intermediate)
		//fmt.Println(intermediate)
		//保存中间值 , 参考reduce写文件
		for _, kv := range intermediate {
			hashKey := ihash(kv.Key) % task.NReduce
			oname := "./mr-mid/mr-mid-"
			mrMidName := oname + strconv.Itoa(hashKey)

			exist, err := PathExists(mrMidName)
			if err != nil {
				return
			}

			var ofile *os.File
			if exist {
				//ofile, _ = os.Open(mrMidName)
				ofile, _ = os.OpenFile(mrMidName, os.O_WRONLY|os.O_APPEND, 0666)
			} else {
				ofile, _ = os.Create(mrMidName)
			}

			str := fmt.Sprintf("%v %v\n", kv.Key, kv.Value)
			ofile.Write([]byte(str))

			ofile.Close()

		}

		//更新任务状态
		uargs := TaskFinishedArgs{}
		ureply := TaskFinishedReply{}

		uargs.Task = task
		uargs.FinishedTime = time.Now()
		uargs.Intermediate = intermediate

		call("Master.UpdateMapTaskFinished", &uargs, &ureply)

		//CallExample()
		//AskForTask()

		time.Sleep(1 * time.Second)
	}

}

func exeMapFun(filename string, mapf func(string, string) []KeyValue, intermediate []KeyValue) []KeyValue {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatalf("cannot open %v", filename)
	}
	content, err := ioutil.ReadAll(file)
	if err != nil {
		log.Fatalf("cannot read %v", filename)
	}
	file.Close()
	kva := mapf(filename, string(content))
	//执行 map
	intermediate = append(intermediate, kva...)
	//kv保存到中间值,很明显是在内存里,这里不考虑爆内存
	return intermediate
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

//
// example function to show how to make an RPC call to the master.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	call("Master.Example", &args, &reply)

	// reply.Y should be 100.
	fmt.Printf("reply.Y %v\n", reply.Y)
}

//
// send an RPC request to the master, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := masterSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
