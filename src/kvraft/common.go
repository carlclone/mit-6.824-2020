package kvraft

const (
	OK             = "OK"
	ErrNoKey       = "ErrNoKey"
	ErrWrongLeader = "ErrWrongLeader"
)

const (
	PUT    = "PUT"
	APPEND = "APPEND"
	GET    = "GET"
)

type Err string

// Put or Append
type PutAppendArgs struct {
	Key       string
	Value     string
	Op        string // "Put" or "Append"
	RequestId int64
	ClientId  int64
}

type PutAppendReply struct {
	Err Err
}

type GetArgs struct {
	Op        string //"GET"
	Key       string
	RequestId int64
	ClientId  int64
}

type GetReply struct {
	Err   Err
	Value string
}
