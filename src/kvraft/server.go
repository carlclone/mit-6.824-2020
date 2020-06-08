package kvraft

import (
	"fmt"
	"labgob"
	"labrpc"
	"log"
	"raft"
	"sync"
	"sync/atomic"
)

const Debug = 1

func DPrintf(format string, a ...interface{}) (n int, err error) {
	if Debug > 0 {
		log.Printf(format, a...)
	}
	return
}

type Op struct {
	// Your definitions here.
	// Field names must start with capital letters,
	// otherwise RPC will break.
	OpName string
	OpKey  string
	OpVal  string
}

type KVServer struct {
	mu      sync.Mutex
	me      int
	rf      *raft.Raft
	applyCh chan raft.ApplyMsg
	dead    int32 // set by Kill()

	maxraftstate int // snapshot if log grows this big

	// Your definitions here.
	kvPairs map[string]string
}

func (kv *KVServer) Get(args *GetArgs, reply *GetReply) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	key := args.Key

	op := Op{}
	op.OpName = "Get"
	op.OpKey = key
	_, _, isLeader := kv.rf.Start(op)
	if !isLeader {
		reply.Err = ErrWrongLeader
		return
	}
	if _, ok := kv.kvPairs[key]; !ok {
		kv.print(LOG_ALL, "server no key %v", kv.kvPairs)
		reply.Err = ErrNoKey
		return
	}
	msg := <-kv.applyCh

	if msg.CommandValid {
		reply.Err = OK
		reply.Value = kv.kvPairs[key]
		return
	}

	reply.Err = ErrWrongLeader
	return
}

func (kv *KVServer) PutAppend(args *PutAppendArgs, reply *PutAppendReply) {
	kv.mu.Lock()
	defer kv.mu.Unlock()

	kv.print(LOG_ALL, "server putAppend start")

	key := args.Key
	val := args.Value

	op := Op{}
	op.OpName = "PutAppend"
	op.OpKey = key
	op.OpVal = val
	_, _, isLeader := kv.rf.Start(op)
	if !isLeader {
		reply.Err = ErrWrongLeader
		return
	}
	kv.print(LOG_ALL, "server putAppend isLeader")
	msg := <-kv.applyCh
	kv.print(LOG_ALL, "server putAppend receive msg from applyCh %v", msg)

	if msg.CommandValid {
		kv.kvPairs[key] = val
		kv.print(LOG_ALL, "server putAppend msg valid %v %v %v", key, val, kv.kvPairs)
		reply.Err = OK
		return
	}

	kv.print(LOG_ALL, "server putAppend msg invalid")
	reply.Err = ErrWrongLeader
	return
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

	kv := new(KVServer)
	kv.me = me
	kv.maxraftstate = maxraftstate

	// You may need initialization code here.

	kv.applyCh = make(chan raft.ApplyMsg)
	kv.rf = raft.Make(servers, me, persister, kv.applyCh)

	// You may need initialization code here.
	kv.kvPairs = make(map[string]string)

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
