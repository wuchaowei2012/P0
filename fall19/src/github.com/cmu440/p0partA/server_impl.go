
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
	// messageQueue     chan []byte  //用于接收结果

	quitSignal_Read  chan int
	quitSignal_Write chan int
}

// Used to specify DBRequests
type db struct {
	qtype QuerryType
	key   string
	value []byte
}

type db_conn struct{
	db_v db
	connection       net.Conn
}

// Implements KeyValueServer.
type keyValueServer struct {

	listener          net.Listener
	currentClients    []*client      //目前的 client 

	newMessage        chan []byte
	newConnection     chan net.Conn

	deadClient        chan *client   //一次只处理一个 deadClient

	// 原始的dbResponse 和 dbQuerry 所有的client共享
	// dbQuery           chan *db      //用以保存 client 的请求，请求处理后发送给 server
	// dbResponse        chan *db

	// 为什么使用指针传结构体的值呢
	// dbQuery_1         map[net.Conn]chan *db_conn
	dbQuery_1         chan *db_conn
	dbResponse_1	  map[net.Conn]chan []byte


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

	kvs.dbQuery_1 = make(chan *db_conn)
	kvs.dbResponse_1 = make(map[net.Conn]chan []byte)

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

	kvs.dbQuery_1= make(chan *db_conn, MAX_MESSAGE_QUEUE_LENGTH)

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
	// kvs.countClients <- 0
	// return <-kvs.clientCount
	fmt.Println("len(kvs.currentClients)", len(kvs.currentClients))
	return len(kvs.currentClients)
}


// 仅仅实现了 伪 接口
func (kvs *keyValueServer) CountActive() int {
	// TODO: implement this!
	// kvs.countClients <- 0
	// return <-kvs.countClients

	fmt.Println("len(kvs.currentClients)", len(kvs.currentClients))
	return len(kvs.currentClients)
}

func (kvs *keyValueServer) CountDropped() int {
	// TODO: implement this!
	// kvs.countClients <- 0
	// return <-kvs.countClients
	return 0
}

// Main server routine.
func runServer(kvs *keyValueServer) {
	defer fmt.Println("\"runServer\" ended.")

	for {
		select {
		// Add a new client to the client list.
		case newConnection := <-kvs.newConnection:
			fmt.Println("create a client ")
			// 为新建立的client 创建两个通道
			
			kvs.dbResponse_1[newConnection] = make(chan []byte, MAX_MESSAGE_QUEUE_LENGTH)

			// 用于管理server的所有clients
			c := &client{
				newConnection,
				make(chan int),
				make(chan int),
			}

			kvs.currentClients = append(kvs.currentClients, c)

			fmt.Println("# of currentClients: ", len(kvs.currentClients))

			go readRoutine(kvs, c)
			go writeRoutine(kvs, c)

		// Remove the dead client.
		case deadClient := <-kvs.deadClient:
			fmt.Println("trying to delete a dead client from kvs.currentClients")
			for i, c := range kvs.currentClients {
				if c == deadClient {
					kvs.currentClients = append(kvs.currentClients[:i], kvs.currentClients[i+1:]...)
					fmt.Println("there are clients left alive:", len(kvs.currentClients))
					break
				}
			}

		case request := <-kvs.dbQuery_1:
			if request.db_v.qtype ==  T_GET{
				v := kvs.store.Get(request.db_v.key)
				
				fmt.Println("dbQuerry type: Get value #:", len(v))
				fmt.Println("dbQuerry key:  :", request.db_v.key)

				for _,item := range v{
					kvs.dbResponse_1[request.connection] <- item
					fmt.Println("getting item:", item)
				}
			} else if request.db_v.qtype ==  T_PUT {
				fmt.Println("dbQuerry type: Put key:", request.db_v.key)

				kvs.store.Put(request.db_v.key, request.db_v.value)
			} else if request.db_v.qtype ==  T_DELETE {
				fmt.Println("dbQuerry type: Delete key:", request.db_v.key)
				kvs.store.Clear(request.db_v.key)
			}

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
			if err == nil {
				kvs.newConnection <- conn
			}
		}
	}
}

// One running instance for each client; reads in
// new  messages and sends them to the server. 读取指令

func readRoutine(kvs *keyValueServer, c *client) {
	defer fmt.Println("\"readRoutine\" ended.")
	clientReader := bufio.NewReader(c.connection)

	for {
		select {
		case <-c.quitSignal_Read:
			return
		default:
			message, err := clientReader.ReadBytes('\n')
			// 为什么读到 EOF的时候，就可以判断某个client 死亡呢？
			if err == io.EOF {
				kvs.deadClient <- c
				fmt.Println("kill a client")
				c.quitSignal_Read <- 0
				c.quitSignal_Write <- 0

			// 普通的 err 不需要杀掉 client，直接return的话， 会有什么影响
			} else if err != nil {
				return
			} else {
				tokens := bytes.Split(message, []byte(":"))
				
				if string(tokens[0]) == "Put" {
					// key := string(tokens[1])
					key := string(bytes.TrimSuffix(tokens[1], []byte("\n")))

					db_v := db{
						qtype: T_PUT, 
						key:   key,
						value: tokens[2],
					}
					kvs.dbQuery_1 <- &db_conn{
						db_v,
						c.connection,
					}

				} else if string(tokens[0]) == "Get" {
					// 高风险的地方
					key := string(bytes.TrimSuffix(tokens[1], []byte("\n")))
					// 首先发送query 请求，然后等待response ---> dbQuery_1 ---> dbResponse_1
					db_v := db{
						qtype: T_GET, 
						key:   key,
					}

					kvs.dbQuery_1 <- &db_conn{
						db_v,
						c.connection,
					}

				} else if string(tokens[0]) == "Delete" {
					key := string(bytes.TrimSuffix(tokens[1], []byte("\n")))
					db_v := db{
						qtype: T_DELETE, 
						key:   key,
					}
					kvs.dbQuery_1 <- &db_conn{
						db_v,
						c.connection,
					}

				}
			}
		}
	}
}

// One running instance for each client; writes messages
// from the message queue to the client.
func writeRoutine(kvs *keyValueServer, c *client) {
	defer fmt.Println("\"writeRoutine\" ended.")

	for {
		select {
			case <-c.quitSignal_Write:
				return
			// c.messageQueue 从client的 messageQueue 中读取数据， messageQueue 从 readRoutine 获取
			// kvs.dbResponse_1[request.connection] <- item
			case item := <- kvs.dbResponse_1[c.connection]:
				_, err := c.connection.Write(item)
				if err != nil{
					fmt.Println("error @ readRoutine")
				}

			}
	}

}