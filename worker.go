package mr

import (
    "encoding/json"
    "fmt"
    "io"
	"log"
    "os"
	"net/rpc"
    "sort"
    "strconv"
    "time"
	"hash/fnv"
)

const sleepTime = 500 * time.Millisecond

// KeyValue
// Map functions return a slice of KeyValue
type KeyValue struct {
    Key   string
    Value string
}
type ByKey []KeyValue

func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return ihash(a[i].Key) < ihash(a[j].Key) }
func ihash(key string) int {
    h := fnv.New32a()
    _, err := h.Write([]byte(key))
    if err != nil {
       return 0
    }
    return int(h.Sum32() & 0x7fffffff)
}

// Worker
// main/mrworker.go calls this function.
func Worker(mapf func(string, string) []KeyValue,
    reducef func(string, []string) string) {

    // Your worker implementation here.
    for {
       timeStamp := time.Now().Unix()
       rpcId := RpcIdT(timeStamp)
       req := ReqArgs{}
       req.ReqId = rpcId
       req.ReqOp = WorkReq 

       res := ResArgs{}
       ok := call("Coordinator.Appoint", &req, &res)
       if !ok {
          log.Println("Maybe Coordinator Server has been closed")
          return
       }

       switch res.ResOp {
       case WorkDone:
          return
       case WorkMap:
          doMap(rpcId, &res, mapf)
       case WorkReduce:
          doReduce(rpcId, &res, reducef)
       case WorkNothing:
          time.Sleep(sleepTime)
       default:
          break
       }
       time.Sleep(sleepTime)
    }
}
func doMap(rpcId RpcIdT, response *ResArgs, mapf func(string, string) []KeyValue) {
    filename := response.ResContent
    file, err := os.Open(filename)
    if err != nil {
       log.Fatalf("cannot open %v", filename)
    }
    defer func(file *os.File) {
       _ = file.Close()
    }(file)
    content, err := io.ReadAll(file)
    if err != nil {
       log.Fatalf("cannot read %v", filename)
    }
    kvs := mapf(filename, string(content))

    ofiles := make([]*os.File, response.ReduceNumN)
    encoders := make([]*json.Encoder, response.ReduceNumN)
    for i := 0; i < response.ReduceNumN; i++ {
       oname := "mr-" + strconv.Itoa(int(response.ResTaskId)) + "-" + strconv.Itoa(i)
       ofiles[i], err = os.Create(oname)
       if err != nil {
          log.Fatal("Can't Create Intermediate File: ", oname)
       }
       defer func(file *os.File, oname string) {
          err := file.Close()
          if err != nil {
             log.Fatal("Can't Close Intermediate File", oname)
          }
       }(ofiles[i], oname)
       encoders[i] = json.NewEncoder(ofiles[i])
    }
    for _, kv := range kvs {
       ri := ihash(kv.Key) % response.ReduceNumN
       err := encoders[ri].Encode(kv)
       if err != nil {
          log.Fatal("Encode Error: ", err)
          return
       }
    }
    req := ReqArgs{
       ReqId:     rpcId,
       ReqOp:     WorkMapDone,
       ReqTaskId: response.ResTaskId,
    }
    res := ResArgs{}
    call("Coordinator.Appoint", &req, &res)
}

func doReduce(rpcId RpcIdT, response *ResArgs, reducef func(string, []string) string) {
    rid := response.ResTaskId 
    var kva []KeyValue
    for i := 0; i < response.MapNumM; i++ {
       func(mapId int) {
          inputName := "mr-" + strconv.Itoa(i) + "-" + strconv.Itoa(int(rid))
          ifile, err := os.Open(inputName)
          if err != nil {
             log.Fatal("Can't open file: ", inputName)
          }
          defer func(file *os.File) {
             err := file.Close()
             if err != nil {
                log.Fatal("Can't close file: ", inputName)
             }
          }(ifile)
          dec := json.NewDecoder(ifile)
          for {
             var kv KeyValue
             if err := dec.Decode(&kv); err != nil {
                break
             }
             kva = append(kva, kv) 
          }
       }(i)
    }
    sort.Sort(ByKey(kva))
    intermediate := kva[:]

    oname := "mr-out-" + strconv.Itoa(int(rid))
    ofile, err := os.Create(oname)
    if err != nil {
       log.Fatal("Can't create file: ", oname)
    }
    defer func(ofile *os.File) {
       err := ofile.Close()
       if err != nil {
          log.Fatal("Can't close file: ", oname)
       }
    }(ofile)
    i := 0
    for i < len(intermediate) {
       j := i + 1
       for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
          j++
       }
       var values []string
       for k := i; k < j; k++ {
          values = append(values, intermediate[k].Value)
       }
       output := reducef(intermediate[i].Key, values)
       _, fprintf := fmt.Fprintf(ofile, "%v %v\n", intermediate[i].Key, output)
       if fprintf != nil {
          return
       }
       i = j
    }
    req := ReqArgs{
       ReqId:     rpcId,
       ReqOp:     WorkReduceDone,
       ReqTaskId: response.ResTaskId,
    }
    res := ResArgs{}
    call("Coordinator.Appoint", &req, &res)
}

// CallExample
// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
func CallExample() {

    // declare an argument structure.
    args := ExampleArgs{}

    // fill in the argument(s).
    args.X = 99

    // declare a reply structure.
    reply := ExampleReply{}

    // send the RPC request, wait for the reply.
    call("Coordinator.Example", &args, &reply)

    // reply.Y should be 100.
    fmt.Printf("reply.Y %v\n", reply.Y)
}

// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
func call(rpcName string, args interface{}, reply interface{}) bool {
    sockname := coordinatorSock()
    c, err := rpc.DialHTTP("unix", sockname)
    if err != nil {
       log.Fatal("dialing:", err)
    }
    defer func(c *rpc.Client) {
       err := c.Close()
       if err != nil {
          log.Fatal("Close Client Error When RPC Calling", err)
       }
    }(c)

    err = c.Call(rpcName, args, reply)
    if err == nil {
       return true
    }

    fmt.Println(err)
    return false
}
