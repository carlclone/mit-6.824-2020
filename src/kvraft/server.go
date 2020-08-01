package kvraft

import (
	"fmt"
	"labgob"
	"labrpc"
	"log"
	"raft"
	"sync"
	"sync/atomic"
	"time"
)

type Op struct {
	Name      string
	Key       string
	Val       string
	RequestId int64
	ClientId  int64
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	kvPairs      map[string]string
	requestCache map[int64]bool  //缓存客户端最后一个请求的结果
	ackNo        map[int64]int64 //保存客户端最后一个确认执行了的requestId，防止重复put 或者append
	pipeMap      map[int]chan Op // raft apply index -> pipe , 真的想不到这种解法，理解都有难度                todo;错误的定义，教训 :保存客户端对应的响应pipe , 因为一个server可以并发处理多个客户端的请求
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	//构造op结构体
	op := Op{}
	op.Name = args.Op
	op.Key = args.Key
	op.RequestId = args.RequestId
	op.ClientId = args.ClientId

	//append op到raft log里
	appendSucc := kv.appendEntryToLog(op)

	//确认log情况
	if !appendSucc {
		reply.Err = ErrWrongLeader
	} else {
		kv.mu.Lock()
		val, keyExist := kv.kvPairs[op.Key]
		kv.mu.Unlock()
		if !keyExist {
			reply.Err = ErrNoKey
		} else {
			reply.Err = OK
			reply.Value = val
		}
	}
	return
}

func (kv *KVServer) appendEntryToLog(op Op) bool {
	index, _, isLeader := kv.rf.Start(op)
	if !isLeader {
		return false
	}
	//是leader，创建和loop thread的管道，监听
	kv.mu.Lock()
	pipe := make(chan Op, 1)
	kv.pipeMap[index] = pipe
	kv.mu.Unlock()

	//增加超时机制
	select {
	case op1 := <-pipe:
		// fix bug , 分区故障恢复后, index 对应的 op 可能不是同一个(被新 leader 的覆盖),要做检查,并让客户端重试
		// 还好 raft 认真做了,细节这么久还记得
		return op == op1
	case <-time.After(1000 * time.Millisecond):
		return false
	}
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	//构造Op结构体
	op := Op{}
	op.Name = args.Op
	op.Key = args.Key
	op.Val = args.Value
	op.RequestId = args.RequestId
	op.ClientId = args.ClientId

	//append 到raft log中
	appendSucc := kv.appendEntryToLog(op)

	//确认log情况
	if !appendSucc {
		reply.Err = ErrWrongLeader
	} else {
		reply.Err = OK
	}
	return
}

func (kv *KVServer) isDup(op Op) bool {
	return kv.ackNo[op.ClientId] >= op.RequestId
}

func (kv *KVServer) updatePair(op Op) {

	switch op.Name {
	case PUT:
		kv.print(LOG_ALL, "updatePair")
		kv.kvPairs[op.Key] = op.Val
	case APPEND:
		kv.print(LOG_ALL, "updatePair")
		kv.kvPairs[op.Key] += op.Val
	}
	kv.print(LOG_ALL, "reqId:%v", op.RequestId)
	kv.ackNo[op.ClientId] = op.RequestId
}

//
// servers[] contains the ports of the set of
// servers that will cooperate via Raft to
// form the fault-tolerant key/value service.
// me is the index of the current server in servers[].
// the k/v server should store snapshots through the underlying Raft
// implementation, which should call persister.SaveStateAndSnapshot() to
// atomically save the Raft state along with the snapshot.
// the k/v server should snapshot when Raft's saved state exceeds maxraftstate bytes,
// in order to allow Raft to garbage-collect its log. if maxraftstate is -1,
// you don't need to snapshot.
// StartKVServer() must return quickly, so it should start goroutines
// for any long-running work.
//
func StartKVServer(servers []*labrpc.ClientEnd, me int, persister *raft.Persister, maxraftstate int) *KVServer {
	// call labgob.Register on structures you want
	// Go's RPC library to marshall/unmarshall.
	DPrintf("kvserver start")
	labgob.Register(Op{})

	//第一版直接加锁串行执行，和redis类似 ，
	//但是这里有一个可以优化的点就是raft复制可以同时复制多个，所以在等待raft完成复制的过程可以从一个一个复制 -> 多个一起复制 ,
	//实现方式是多个请求一起等待server响应，而不是阻塞

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	// You may need initialization code here.
	kv.kvPairs = make(map[string]string)
	kv.requestCache = make(map[int64]bool)
	kv.ackNo = make(map[int64]int64)
	kv.pipeMap = make(map[int]chan Op)

	//开启loop thread , 监听applyCh
	go func() {
		for {
			msg := <-kv.applyCh
			//取出obj,强转成Op
			op := msg.Command.(Op)

			//如果重复了就不执行put append , 这里主要是幂等问题，get天然幂等，关注重复是否有害
			kv.mu.Lock()
			if !kv.isDup(op) {
				kv.updatePair(op)
			}
			//取出pipe,返回结果
			// 有没有这种可能： raft复制的太快了，pipe还没创建完就被applyCh读到了
			pipe, ok := kv.pipeMap[msg.CommandIndex]
			kv.mu.Unlock()
			if ok {
				kv.print(LOG_ALL, "取到msg %v", msg)

				pipe <- op
			}

		}
	}()

	return kv
}

//
// the tester calls Kill() when a KVServer instance won't
// be needed again. for your convenience, we supply
// code to set rf.dead (without needing a lock),
// and a killed() method to test rf.dead in
// long-running loops. you can also add your own
// code to Kill(). you're not required to do anything
// about this, but it may be convenient (for example)
// to suppress debug output from a Kill()ed instance.
//
func (kv *KVServer) Kill() {
	atomic.StoreInt32(&kv.dead, 1)
	kv.rf.Kill()
	// Your code here, if desired.
}

func (kv *KVServer) killed() bool {
	z := atomic.LoadInt32(&kv.dead)
	return z == 1
}

const Debug = 1

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}

func (kv *KVServer) print(level int, format string, a ...interface{}) {
	//m := map[int]bool{
	//LOG_ALL:       true,
	//LOG_VOTE:      true,
	//LOG_HEARTBEAT: true,
	//LOG_REPLICA_1: true,
	//LOG_PERSIST: false,
	//LOG_UN8:     true,
	//}
	//if !m[level] {
	//	return
	//}

	//m2 := []string{"leader", "candidate", "follower"}

	format = fmt.Sprintf("SERVER#%v  - %v", kv.me, format)
	DPrintf(format, a...)
}
