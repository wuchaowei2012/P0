
package p0partA

 // Implementation of a KeyValueServer. Students should write their code in this file.
// Implementation of a KeyValueServer.

import (
	"github.com/cmu440/p0partA/kvstore"
	"fmt"
	"net"
	"strconv"
	"bufio"
	"bytes"
	"io"
)

const MAX_MESSAGE_QUEUE_LENGTH = 500

type QuerryType int

const (
	T_PUT       QuerryType = 0
	T_GET       QuerryType = 1
	T_DELETE    QuerryType = 2
)

// Stores a connection and corresponding message queue.
type client struct {
	connection       net.Conn
	messageQueue     chan []byte  //用于接收结果

	cQuery chan *db               //查询是不需要 block 
	cResponse chan *db

	quitSignal_Read  chan int
	quitSignal_Write chan int
}

// Used to specify DBRequests
type db struct {
	qtype int
	key   string
	value []byte
}

// Implements KeyValueServer.
type keyValueServer struct {

	listener          net.Listener
	currentClients    []*client      //目前的 client 

	newMessage        chan []byte
	newConnection     chan net.Conn

	deadClient        chan *client   //一次只处理一个 deadClient

	// 原始的dbResponse 和 dbQuerry 所有的client共享
	dbQuery           chan *db      //用以保存 client 的请求，请求处理后发送给 server
	dbResponse        chan *db

	dbQuery_1         map[net.Conn]chan *db
	dbResponse_1	  map[net.Conn]chan *db

	countClients      chan int   //这两个有什么区别
	clientCount       chan int

	quitSignal_Main   chan int   //通过通道的方式发送信号给routine， 通知其结束
	quitSignal_Accept chan int   //通过通道的方式发送信号给routine， 通知其结束

	store kvstore.KVStore  // kvstore的具体实现
}

// Initializes a new KeyValueServer.
func New(store kvstore.KVStore) KeyValueServer {
	
	var kvs keyValueServer

	kvs.newMessage = make(chan []byte)
	kvs.newConnection = make(chan net.Conn)

	kvs.deadClient=make(chan *client)

	kvs.dbQuery=make(chan *db)
	kvs.dbResponse=make(chan *db)

	kvs.dbQuery_1 = make(map[net.Conn]chan *db)
	kvs.dbResponse_1 = make(map[net.Conn]chan *db)

	kvs.countClients= make(chan int) 
	kvs.clientCount= make(chan int)

	kvs.quitSignal_Main=make(chan int)   
	
	kvs.quitSignal_Accept= make(chan int)
	kvs.store = store

	// 使用接口时，返回接口类型变量， 参考 book p113
	return &kvs

}

// Implementation of Start for keyValueServer.
func (kvs *keyValueServer) Start(port int) error {
	ln, err := net.Listen("tcp", ":"+strconv.Itoa(port))
	if err != nil {
		return err
	}

	kvs.listener = ln
	// init_db()

	go runServer(kvs)
	go acceptRoutine(kvs)

	return nil
}

// Implementation of Close for keyValueServer.
func (kvs *keyValueServer) Close() {
	kvs.listener.Close()
	kvs.quitSignal_Main <- 0
	kvs.quitSignal_Accept <- 0
}

// Implementation of Count.
func (kvs *keyValueServer) Count() int {
	kvs.countClients <- 0
	return <-kvs.clientCount
}


// 仅仅实现了 伪 接口
func (kvs *keyValueServer) CountActive() int {
	// TODO: implement this!
	kvs.countClients <- 0
	return 0
}

func (kvs *keyValueServer) CountDropped() int {
	// TODO: implement this!

	kvs.countClients <- 0
	return 0
}

// Main server routine.
func runServer(kvs *keyValueServer) {
	// defer fmt.Println("\"runServer\" ended.")

	for {
		select {
		// Send the message to each client's queue.
		case newMessage := <-kvs.newMessage:
			for _, c := range kvs.currentClients {
				// If the queue is full, drop the oldest message.
				// 确定下是 丢掉最老的，还是最新的数据
				if len(c.messageQueue) == MAX_MESSAGE_QUEUE_LENGTH {
					<-c.messageQueue
				}
				// 所有的client 接收的信息都是 相同的
				c.messageQueue <- newMessage
			}
		// Add a new client to the client list.


		case newConnection := <-kvs.newConnection:
			fmt.Println("create a client ")
			c := &client{
				newConnection,
				make(chan []byte, MAX_MESSAGE_QUEUE_LENGTH),
				make(chan *db),
				make(chan *db, MAX_MESSAGE_QUEUE_LENGTH),
				make(chan int),
				make(chan int)}
			kvs.currentClients = append(kvs.currentClients, c)
			fmt.Println("# of currentClients", len(kvs.currentClients))
			go readRoutine(kvs, c)
			go writeRoutine(c)

		// Remove the dead client.
		case deadClient := <-kvs.deadClient:
			for i, c := range kvs.currentClients {
				if c == deadClient {
					kvs.currentClients =
						append(kvs.currentClients[:i], kvs.currentClients[i+1:]...)
					break
				}
			}

		// Run a query on the DB
		case request := <-kvs.dbQuery:
			// response required for GET query
			// if request.isGet {
			// 	v := kvs.store.Get(request.key)
			// 	for _,item := range v{
			// 		kvs.dbResponse <- &db{
			// 			value: item,
			// 		}
			// 	}
			// } else {
			// 	kvs.store.Put(request.key, request.value)
			// }
			x := 1

		// Get the number of clients.
		case <-kvs.countClients:
			kvs.clientCount <- len(kvs.currentClients)

		// End each client routine.
		case <-kvs.quitSignal_Main:
			for _, c := range kvs.currentClients {
				c.connection.Close()
				c.quitSignal_Write <- 0
				c.quitSignal_Read <- 0
			}
			
		return

		}
	}
}

// One running instance; accepts new clients and sends them to the server.
func acceptRoutine(kvs *keyValueServer) {
	defer fmt.Println("\"acceptRoutine\" ended.")

	for {
		select {
		case <-kvs.quitSignal_Accept:
			return
		default:
			conn, err := kvs.listener.Accept()
			//为每个新建的 client 创建两个用于 数据查询 的通道

			// dbQuery_1 map[net.Conn]chan *db
			// dbResponse_1 map[net.Conn]chan *db
			kvs.dbQuery_1[conn] = make(chan []byte, sendChannelBufferSize)

			if err == nil {
				kvs.newConnection <- conn
			}
		}
	}
}

// One running instance for each client; reads in
// new  messages and sends them to the server.

func readRoutine(kvs *keyValueServer, c *client) {
	defer fmt.Println("\"readRoutine\" ended.")

	clientReader := bufio.NewReader(c.connection)

	// Read in messages.
	for {
		select {
		case <-c.quitSignal_Read:
			return
		default:
			message, err := clientReader.ReadBytes('\n')
			// 为什么读到 EOF的时候，就可以判断某个client 死亡呢？
			if err == io.EOF {
				kvs.deadClient <- c
			// 普通的 err 不需要杀掉 client，直接return的话， 会有什么影响
			} else if err != nil {
				return
			} else {
				tokens := bytes.Split(message, []byte(":"))
				if string(tokens[0]) == "Put" {
					key := string(tokens[1][:])

					// do a "put" query
					kvs.dbQuery <- &db{
						qtype: T_PUT,
						key:   key,
						value: tokens[2],
					}
				} else if string(tokens[0]) == "Get" {
					// remove trailing \n from get,key\n request
					keyBin := tokens[1][:len(tokens[1])-1]
					key := string(keyBin[:])
					
					// 首先发送 去query 请求，然后等待response
					// do a "get" query
					kvs.dbQuery <- &db{
						qtype: T_GET,
						key:   key,
					}

					response := <-kvs.dbResponse
					// 后面跟的 ... 代表什么意思
					// 原始的代码里面 是将同一信息发送给 所有的client端，实际上不需要这样
					// kvs.newMessage <- append(append(keyBin, ":"...), response.value...)
					// 个性化， 数据仅发送给需要的 client 里面
					c.messageQueue <- append(append(keyBin, ":"...), response.value...)

				} else if string(tokens[0]) == "Delete" {
					keyBin := tokens[1][:len(tokens[1])-1]
					key := string(keyBin[:])

					kvs.dbQuery <- &db{
						qtype: T_DELETE,
						key:   key,
					}
				}
			}
		}
	}
}

// One running instance for each client; writes messages
// from the message queue to the client.
func writeRoutine(c *client) {
	defer fmt.Println("\"writeRoutine\" ended.")

	for {
		select {
		case <-c.quitSignal_Write:
			return
		// c.messageQueue 从client的 messageQueue 中读取数据， messageQueue 从 readRoutine 获取
		case message := <-c.messageQueue:
			fmt.Println("message in client messageQueue from readRoutine: ", string(message))
			item:=append(message, (byte)('\n'))
			// item=append(item, '\n')
			_, err := c.connection.Write(item)
			if err != nil{
				fmt.Println("error @ readRoutine")
			}

		}
	}

}