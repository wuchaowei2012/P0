// Implementation of a KeyValueServer. Students should write their code in this file.

package p0partA

import (
	"github.com/cmu440/p0partA/kvstore"
	 "fmt"
	 "net"
	  "strconv"
	"bufio"
	"time"
)

// 整个函数的思路是 将client 对应的 conn 信息读到 server的 readChan
// server的 readChan信息依次每个 client 使用的通道里
// server 将 client对应通道中的信息读出来，并写入到 client对应的conn

const sendChannelBufferSize int = 100

type keyValueServer struct {
	// TODO: implement this!
	clientNum int
	listener net.Listener
	// 用于 维护 conn 和 对应的通道
	channelMap map[net.Conn]chan []byte
	// 用于保存 conn 发送过来的msg 的通道
	readChan chan []byte
}

// New creates and returns (but does not start) a new KeyValueServer.
func New(store kvstore.KVStore) KeyValueServer {
	// TODO: implement this!
	var server keyValueServer
	
	server.clientNum = 0
	server.listener = nil
	server.readChan = make(chan []byte)
	server.channelMap = make(map[net.Conn]chan []byte)
	
	// 使用接口时，返回接口类型变量， 参考 book p113
	return &server
}

func (kvs *keyValueServer) Start(port int) error {
	// TODO: implement this!
	laddr := ":" + strconv.Itoa(port)

	// 初始化 监听本地服务, 并将变量赋值给 kvs
	listener, err := net.Listen("tcp", laddr)

	// A Listener is a generic network listener for stream-oriented protocols.
	// Multiple goroutines may invoke methods on a Listener simultaneously.
	// type Listener interface {
	//     // Accept waits for and returns the next connection to the listener.
	//     Accept() (Conn, error)

	//     // Close closes the listener.
	//     // Any blocked Accept operations will be unblocked and return errors.
	//     Close() error

	//     // Addr returns the listener's network address.
	//     Addr() Addr
	// }

	if err != nil {
		fmt.Println("error in starting a listener!")
		return nil
	}

	kvs.listener = listener
	// 进行response 的处理， 也就是 核心代码

	go kvs.handleOutStuff()

	go func(){
        for{
			// Conn is a generic stream-oriented network connection.
			// Accept waits for and returns the next connection to the listener.
			// Accept() (Conn, error)
            conn,err := kvs.listener.Accept()
            if err != nil{
                return
			}
			// 新建一个进程意味着，增加一个client
			fmt.Println("has already added a client")
			kvs.clientNum++

			// 解析 connection 请求
            go kvs.readStuff(conn)
        }
    }()

	return nil
}

func (kvs *keyValueServer) readStuff(conn net.Conn){
	// 为什么要将 conn 杀掉
	defer conn.Close()

	bufReader := bufio.NewReader(conn)
	// 根据某个新建 conn 建立一个 chan, 用于 ? 
	kvs.channelMap[conn] = make(chan []byte, sendChannelBufferSize)

	// 处理 client 发送过来的请求
	go kvs.sendStuff(conn)

	// 根据conn 发送response
	for{
		// func (b *Reader) ReadBytes(delim byte) ([]byte, error)
		// ReadBytes reads until the first occurrence of delim in the input, 
		// returning a slice containing the data up to and including the delimiter. 
		// If ReadBytes encounters an error before finding a delimiter, 
		// it returns the data read before the error and the error itself (often io.EOF). 
		// ReadBytes returns err != nil if and only if the returned data does not end in delim.
		// For simple uses, a Scanner may be more convenient.

		line, err := bufReader.ReadBytes('\n')

		if err != nil{
			fmt.Println("erro @ ReadBytes_parseResq")
			delete(kvs.channelMap, conn)
			kvs.clientNum--
			break
		}
		// 写给服务器
		// 为什么长度总是 0
		// fmt.Println("readStuff:kvs.readChan:", len(kvs.readChan))
		kvs.readChan<-line

	}
}

func (kvs *keyValueServer) sendStuff(conn net.Conn){
	// channelMap map[net.Conn] chan []byte
	sendChannel, exists := kvs.channelMap[conn]
	if !exists{
		fmt.Println("error @ sendStuff: error when get value by key")
		return
	}
	for{
		// 从 channel 中 读数据，读到msg变量中, 发送回 client
		msg := <- sendChannel
		// conn 可以read 也可以 write, interesting 
		_, err := conn.Write(msg)
		if err != nil{
			delete(kvs.channelMap, conn)
		}
	}
}


func (kvs *keyValueServer) handleOutStuff(){
	for {
		msg := <-kvs.readChan
		fmt.Println("handleOutStuff:total clients:",len(kvs.channelMap))
		for _, sendChan := range kvs.channelMap{
			// buffer channedl 的处理很新奇
			if len(sendChan) < sendChannelBufferSize{
				sendChan <-msg
			} else {
				time.Sleep(time.Duration(10)*time.Millisecond)}
		}
	}
}


func (kvs *keyValueServer) Close() {
	// TODO: implement this!
	if kvs.listener != nil{
		kvs.listener.Close()
	}
	kvs.clientNum = 0
}

func (kvs *keyValueServer) CountActive() int {
	// TODO: implement this!
	fmt.Println("in CountActive function",kvs.clientNum)
	return kvs.clientNum
}

func (kvs *keyValueServer) CountDropped() int {
	// TODO: implement this!

	fmt.Println("in CountDropped function",kvs.clientNum)
	return kvs.clientNum
}

// TODO: add additional methods/functions below!
