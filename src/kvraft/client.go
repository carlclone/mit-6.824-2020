package kvraft

import (
	"labrpc"
)
import "crypto/rand"
import "math/big"

type Clerk struct {
	servers []*labrpc.ClientEnd
	// You will have to modify this struct.
	id        int64
	nextReqId int64
}

func nrand() int64 {
	max := big.NewInt(int64(1) << 62)
	bigx, _ := rand.Int(rand.Reader, max)
	x := bigx.Int64()
	return x
}

func MakeClerk(servers []*labrpc.ClientEnd) *Clerk {
	ck := new(Clerk)
	ck.servers = servers
	ck.id = nrand()
	ck.nextReqId = 1
	ck.print(LOG_ALL, "client init")
	// You'll have to add code here.
	return ck
}

func (ck *Clerk) genReqId() int64 {
	reqId := ck.nextReqId
	ck.nextReqId++
	return reqId
}

//
// fetch the current value for a key.
// returns "" if the key does not exist.
// keeps trying forever in the face of all other errors.
func (ck *Clerk) Get(key string) string {

	ck.print(LOG_ALL, "client get %v", key)
	args := GetArgs{}
	args.Key = key
	args.ClientId = ck.id
	args.RequestId = ck.genReqId()
	args.Op = GET

	for {
		for i, _ := range ck.servers {
			reply := GetReply{}
			ok := ck.servers[i].Call("KVServer.Get", &args, &reply)
			if ok {
				if reply.Err == ErrNoKey {
					ck.print(LOG_ALL, "client errnokey")
					return ""
				}
				if reply.Err == OK {
					ck.print(LOG_ALL, "client get ok")
					return reply.Value
				}
			}
		}
	}
}

//
// shared by Put and Append.
//
func (ck *Clerk) PutAppend(key string, value string, op string) {
	ck.print(LOG_ALL, "client putAppend %v %v %v", key, value, op)
	args := PutAppendArgs{}
	args.Key = key
	args.Value = value
	args.Op = op
	args.ClientId = ck.id
	args.RequestId = ck.genReqId()
	for {
		for i, _ := range ck.servers {
			reply := PutAppendReply{}
			ok := ck.servers[i].Call("KVServer.PutAppend", &args, &reply)
			//ck.print(LOG_ALL, "client putAppend ok:%v", ok)
			if ok {
				if reply.Err == OK {
					ck.print(LOG_ALL, "client putAppend ok k%v v%v", key, value)
					return
				}
			}
		}
	}
}

func (ck *Clerk) Put(key string, value string) {
	ck.PutAppend(key, value, PUT)
}
func (ck *Clerk) Append(key string, value string) {
	ck.PutAppend(key, value, APPEND)
}

const (
	LOG_ALL = 1
)

func (ck *Clerk) print(level int, format string, a ...interface{}) {
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

	//format = fmt.Sprintf("SERVER#%v ROLE#%v TERM#%v - %v", rf.me, m2[rf.role-1], rf.currentTerm, format)
	DPrintf(format, a...)
}
