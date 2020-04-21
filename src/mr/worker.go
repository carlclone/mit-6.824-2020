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

	//var task *Task
	for {
		//获取一个任务
		args := AskForTaskArgs{}
		reply := AskForTaskReply{}
		call("Master.RetrieveTask", &args, &reply)
		//fmt.Println(reply.Status)
		if reply.Status == ASK_FOR_TASK_FAIL {
			time.Sleep(1 * time.Second)
			continue
		}

		if reply.Status == ASK_FOR_TASK_DONE {
			break
		}

		task := reply.Task

		if task.Type == TYPE_REDUCE {

			filename := task.FileName
			//fmt.Println(task.FileName)

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
			sort.Sort(ByKey(intermediate))

			//taskNum:=strings.Split(filename,"-")
			last1 := filename[len(filename)-1:]
			oname := "mr-out-" + last1
			ofile, _ := os.Create(oname)

			//
			// call Reduce on each distinct key in intermediate[],
			// and print the result to mr-out-0.
			//
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
				fmt.Fprintf(ofile, "%v %v\n", intermediate[i].Key, output)

				i = j
			}

			ofile.Close()

			//更新任务状态
			uargs := TaskFinishedArgs{}
			ureply := TaskFinishedReply{}

			uargs.Task = task
			uargs.FinishedTime = time.Now()
			//uargs.Intermediate = intermediate

			call("Master.UpdateMapTaskFinished", &uargs, &ureply)

			time.Sleep(1 * time.Second)
			continue
		}

		filename := task.FileName
		intermediate := []KeyValue{}

		//fmt.Println(task.FileName)
		//执行任务 , 参考sequential
		intermediate = exeMapFun(filename, mapf, intermediate)
		//fmt.Println(intermediate)
		//保存中间值 , 参考reduce写文件
		for _, kv := range intermediate {
			hashKey := ihash(kv.Key) % task.NReduce
			oname := "./mr-mid-"
			mrMidName := oname + strconv.Itoa(hashKey)

			exist, err := PathExists(mrMidName)
			if err != nil {
				panic(err)
				return
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
