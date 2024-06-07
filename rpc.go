package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import "os"
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
type RpcIdT int64 
type ReqArgs struct {
    ReqId     RpcIdT
    ReqOp     WorkType
    ReqTaskId TaskIdT
}

type ResArgs struct {
    ResId      RpcIdT
    ResOp      WorkType
    ResTaskId  TaskIdT 
    ResContent string
    ReduceNumN int 
    MapNumM    int 
}
type WorkType int
type TaskIdT int

const (
    WorkNothing    WorkType = iota
    WorkReq                 
    WorkMap                 
    WorkReduce              
    WorkDone                
    WorkTerminate          
    WorkMapDone           
    WorkReduceDone  
)

type Rpc struct {
    Req ReqArgs
    Res ResArgs
}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
    s := "/var/tmp/824-mr-"
    s += strconv.Itoa(os.Getuid())
    return s
}
