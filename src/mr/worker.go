package mr

import (
	"fmt"
	"sort"
	"strings"
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

// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	for {
		//获取一个任务
		reply := GetTask()
		//获取失败
		if reply.Status == ASK_FOR_TASK_FAIL {
			time.Sleep(1 * time.Second)
			continue
		}
		//master 已经 Done , worker 退出
		if reply.Status == ASK_FOR_TASK_DONE {
			break
		}

		task := reply.Task

		switch task.Type {
		case TYPE_REDUCE:
			filename := task.FileName
			fmt.Println(task.FileName)
			//从文件读取intermediate []KeyValue
			file_bytes, err := ioutil.ReadFile(filename)
			if err != nil {
				panic(err)
			}
			lines := strings.Split(string(file_bytes), "\n")
			intermediate := []KeyValue{}
			for _, line := range lines {
				items := strings.Split(line, " ")
				if len(items) == 1 {
					continue
				}
				intermediate = append(intermediate, KeyValue{Key: items[0], Value: items[1]})
			}
			//执行任务,参考sequential
			reduceOutPut := ExecuteReducef(intermediate, reducef)

			taskExecuted := NoticeTaskFinished(task)
			if !taskExecuted {
				//保存输出
				SaveReduceOutPut(filename, reduceOutPut)
			}
		case TYPE_MAP:
			filename := task.FileName
			intermediate := []KeyValue{}
			//fmt.Println(task.FileName)
			//执行任务 , 参考sequential
			intermediate = exeMapFun(filename, mapf, intermediate)
			taskExecuted := NoticeTaskFinished(task)
			if !taskExecuted {
				//保存中间值 , 参考reduce写文件
				SaveIntermediate(intermediate, task)

			}

		}

		time.Sleep(1 * time.Second)

	}

}

func SaveReduceOutPut(filename string, reduceOutPut string) {
	last1 := filename[len(filename)-1:]
	oname := "mr-out-" + last1
	ofile, _ := os.Create(oname)
	ofile.Write([]byte(reduceOutPut))
	ofile.Close()
}

func ExecuteReducef(intermediate []KeyValue, reducef func(string, []string) string) string {
	sort.Sort(ByKey(intermediate))
	//taskNum:=strings.Split(filename,"-")
	//
	// call Reduce on each distinct key in intermediate[],
	// and print the result to mr-out-0.
	//
	reduceOutPut := ""
	i := 0
	for i < len(intermediate) { //遍历每个中间值
		j := i + 1
		//找到某个 key 的个数 j-i 个
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value) //把每个中间值的 value 放进去 , 这个 case 里全是 1
		}
		output := reducef(intermediate[i].Key, values) //数有多少个 1

		// this is the correct format for each line of Reduce output.
		//fmt.Println(output)
		reduceOutPut += fmt.Sprintf("%v %v\n", intermediate[i].Key, output)

		i = j
	}
	return reduceOutPut
}

func GetTask() AskForTaskReply {
	args := AskForTaskArgs{}
	reply := AskForTaskReply{}
	call("Master.RetrieveTask", &args, &reply)
	fmt.Println("reply_status:" + strconv.Itoa(reply.Status))
	return reply
}

func GetIsTaskExecuted(task *Task) bool {
	args := TaskExecutedArgs{}
	args.Task = task
	reply := TaskExecutedReply{}
	call("Master.IsTaskExecuted", &args, &reply)
	fmt.Println("task_executed_status:" + strconv.Itoa(reply.Status))
	if reply.Status == TASK_ALREADY_EXECUTED {
		return true
	}
	return false
}

func SaveIntermediate(intermediate []KeyValue, task *Task) {
	//先把相同key的放到同一个数组里 ,  生成相同hashKey的字符串 , 再一并写入
	tmpSlice := make([][]KeyValue, task.NReduce)
	for _, kv := range intermediate {
		hashKey := ihash(kv.Key) % task.NReduce
		tmpSlice[hashKey] = append(tmpSlice[hashKey], kv)
	}

	tmpOuputSlice := []string{}
	for _, v1 := range tmpSlice {
		tmpStr := ""
		for _, v2 := range v1 {
			tmpStr += fmt.Sprintf("%v %v\n", v2.Key, v2.Value)
		}
		tmpOuputSlice = append(tmpOuputSlice, tmpStr)
	}

	for hashKey, v := range tmpOuputSlice {
		oname := "./mr-mid-"
		mrMidName := oname + strconv.Itoa(hashKey)

		exist, err := PathExists(mrMidName)
		if err != nil {
			panic(err)
			//return
		}

		var ofile *os.File
		if exist {
			//ofile, _ = os.Open(mrMidName)
			ofile, err = os.OpenFile(mrMidName, os.O_WRONLY|os.O_APPEND, 0666)
			if err != nil {
				panic(err)
			}
		} else {
			ofile, err = os.Create(mrMidName)
			if err != nil {
				panic(err)
			}
		}

		ofile.Write([]byte(v))

		ofile.Close()
	}
}

func NoticeTaskFinished(task *Task) bool {
	uargs := TaskFinishedArgs{}
	ureply := TaskFinishedReply{}
	uargs.Task = task
	uargs.FinishedTime = time.Now()
	//uargs.Intermediate = intermediate
	call("Master.UpdateTaskFinished", &uargs, &ureply)

	if ureply.Status == TASK_ALREADY_EXECUTED {
		return true
	}
	return false
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
