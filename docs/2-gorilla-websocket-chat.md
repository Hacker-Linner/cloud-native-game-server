# Nano 分析前奏之跟随 Gorilla WebSocket 示例再过一遍 Golang 的并发编程

## 介绍

### 这是一个系列

1. [探索 Golang 云原生游戏服务器开发，5 分钟上手 Nano 游戏服务器框架](https://juejin.im/post/6870388583019872270)

### Gorilla WebSocket 是什么？

Gorilla WebSocket 是 WebSocket 协议的 Go 实现。

WebSocket 是啥？为少这里就不赘述了，掘友们在[掘金](https://juejin.im/search?query=websocket&type=all)上科普了太多太多😂。

### 示例仓库
* 官方例子：[Chat example](https://github.com/gorilla/websocket/tree/master/examples/chat)
* 为上更改过的例子：[cloud-native-game-server/2-gorilla-websocket-chat](https://github.com/Hacker-Linner/cloud-native-game-server/tree/master/demo/2-gorilla-websocket-chat)

### 为啥要再熟悉下这个例子？

**通过通信共享内存**，**通过通信共享内存**，**通过通信共享内存**

分析 Nano 之前，再过一遍 Golang 的并发编程。

## 示例分析

这里我整理下这个例子的官方 [README.md](https://github.com/gorilla/websocket/tree/master/examples/chat)

### 一句话描述业务

1. 客户端可以连接服务器
2. 客户端可以发送消息，然后服务端立即广播消息

### 技术描述业务

本质上，就是对多个 `websocket` 连接的管理和读写操作。

1. 服务端向客户端发送消息，技术上就是客户端的 `websocket` 连接进行 `读` 和 `写` 操作。
  * 这里就抽象出来的 `Client`，里面有自己这个 `websocket` 连接的 `读` 和 `写` 操作
2. 多个客户端，就是说多个 `websocket` 的维护工作。
  * 这里就抽象出来的 `Hub`，它维护着所有的 `Client`，广播的无非就是调用 `Client` 里面的 `websocket` 连接的 `写` 操作

### Server

服务器应用程序定义两种类型，`Client` 和 `Hub`。服务器为每个 websocket 连接创建一个 `Client` 类型的实例。
`Client` 充当 websocket 连接和 `Hub` 类型的单个实例之间的中介。`Hub` 维护一组注册的客户端，并向客户端广播消息。

应用程序为 `Hub` 运行一个 goroutine，为每个 `Client` 运行两个 goroutine。多个 goroutine 使用通道相互通信。该 `Hub` 有用于注册客户端、注销客户端和广播消息的通道。`Client` 有一个缓冲的出站消息通道。客户端的 goroutine 之一从该通道读取消息，并将消息写入 websocket。另一个客户端 goroutine 从 websocket 读取消息并将其发送到 hub。

核心源码解释：
```go
......
func main() {
  ......
  // 应用一运行，就初始化 `Hub` 管理工作
  hub := newHub()
  // 开个 goroutine，后台运行监听三个 channel
  // register：注册客户端 channel
  // unregister：注销客户端 channel
  // broadcast：广播客户端 channel
  go hub.run()
  
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWs(hub, w, r)
  })
  .....
}
......
// serveWs 处理来自每一个客户端的 "/ws" 请求。
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
  // 升级这个请求为 `websocket` 协议
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
  }
  // 初始化当前的客户端实例，并与 `hub` 中心管理勾搭上，
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

  // 通过在新的goroutines中完成所有工作，允许调用者引用内存的集合。
  // 其实对当前 `websocket` 连接的 `I/O` 操作
  // 写操作（发消息到客户端）-> 这里 `Hub` 会统一处理
  go client.writePump()
  // 读操作（对消息到客户端）-> 读完当前连接立即发 -> 交由 `Hub` 分发消息到所有连接
	go client.readPump()
}
```

### Hub

`Hub` 类型的代码在 [hub.go](https://github.com/Hacker-Linner/cloud-native-game-server/blob/master/demo/2-gorilla-websocket-chat/hub.go) 中。应用程序的 `main` 函数将以 goroutine 的形式启动 hub 的 `run` 方法。客户端使用 `register`、`unregister` 和 `broadcast` 通道向 hub 发送请求。

hub 通过在 `clients` map 中添加 client 指针作为键来注册客户端。map 值始终为真。

注销代码稍微复杂一点。除了从 `clients` map 中删除 client 指针外，hub 还关闭了客户端的 `send` 通道，向客户端发出信号，表示不会再向客户端发送任何消息。

hub 通过在已注册的客户端上循环并将消息发送到客户端的 `send` 通道来处理消息。如果客户端的 `send` 缓冲区已满，则hub 会假定客户端已死或卡住。在本例中，hub 注销客户端并关闭 websocket。

核心源码解释：

```go
func (h *Hub) run() {
	for {
		select {
		// 注册 channel
    case client := <-h.register:
      // 键值对操作，没啥好说的
			h.clients[client] = true
		// 注销 channel
    case client := <-h.unregister:
      // 键值对操作，没啥好说的
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		// 广播 channel
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
        // 直接送入各个连接的 send channel
        case client.send <- message:
        // 卡住，这里直接踢掉
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
```

### Client

`Client` 类型的代码在 [client.go](https://github.com/Hacker-Linner/cloud-native-game-server/blob/master/demo/2-gorilla-websocket-chat/client.go) 中。

`serveWs` 函数由应用程序的 `main` 函数注册为 HTTP 处理程序。处理程序将 HTTP 连接升级到 WebSocket 协议，创建一个 client，在 hub 上注册 client，并使用 defer 语句计划将客户端注销。

接下来，HTTP 处理程序启动 client 的 `writePump` 方法作为一个 goroutine。这个方法将消息从 client 的 send 通道传输到 websocket 连接。当 hub 关闭通道或者在 websocket 连接上写入错误时，writer 方法退出。

最后，HTTP 处理程序调用客户端的 `readPump` 方法。这个方法从 websocket 传输入站消息到 hub。

WebSocket 连接 [支持一个并发读取器和一个并发写入器](https://godoc.org/github.com/gorilla/websocket#hdr-Concurrency)。该应用程序通过执行对 `readPump` goroutine 的所有读取和对 `writePump` goroutine 的所有写入来确保满足这些并发要求。

为了提高高负载下的效率，`writePump` 函数将 `send` 通道中等待的聊天消息合并为一个单一的 WebSocket 消息。这减少了系统调用的数量和通过网络发送的数据量。

核心源码解释：

```go
// readPump 从 Websocket 连接用泵将消息输送到 hub。
// 应用程序在每个连接 goroutine 中运行 readPump。
// 应用程序通过执行此 goroutine 中的所有读取来确保连接上最多有一个 reader。
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()
	// SetReadLimit 设置从对等方读取的消息的最大大小。如果消息超出限制，则连接会将关闭消息发送给对等方，然后将ErrReadLimit返回给应用程序。
	c.conn.SetReadLimit(maxMessageSize)
	// SetReadDeadline 设置基础网络连接上的读取期限。读取超时后，websocket 连接状态已损坏，以后所有读取将返回错误。参数值为零表示读取不会超时。
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	// SetPongHandler 为从 peer 接收到的 pong 消息设置处理程序。处理程序的参数是 PONG 消息应用程序数据。默认的 pong 处理程序不执行任何操作。
	// handler函数从 NextReader、ReadMessage 和 message reader Read方法处被调用。
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	for {
		// 读取消息
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			// 错误处理
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("error: %v", err)
			}
			break
		}
		// 整理 message 内容
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))

		// 广播
		c.hub.broadcast <- message
	}
}

// writePump 将消息从 hub pump到 websocket 连接。

// 为每个连接启动运行 writePump 的 goroutine。
// 通过执行这个 goroutine 中的所有写操作，应用程序确保连接最多只有一个 writer。
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()
	for {
		select {
		// 写消息到当前的 websocket 连接
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// hub 关闭这个 channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			// NextWriter 为要发送的下一条消息返回一个写入器。写入器的Close方法将完整的消息刷新到网络。
			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)
			// 将排队聊天消息添加到当前的 websocket 消息中。
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write(newline)
				w.Write(<-c.send)
			}
			if err := w.Close(); err != nil {
				return
			}
		// 定时检测下客户端的状态
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
```

**API 的相关细节，大家可以直接查文档，思想才是最重要的**

### Frontend

前端代码在 [home.html](https://github.com/Hacker-Linner/cloud-native-game-server/blob/master/demo/2-gorilla-websocket-chat/home.html) 中。

在加载文档时，脚本在浏览器中检查 websocket 功能。如果 websocket 功能可用，那么脚本打开一个到服务器的连接，并注册一个回调函数来处理来自服务器的消息。回调函数使用 appendLog 函数将消息追加到聊天日志中。

为了允许用户手动滚动聊天日志而不受新消息的干扰，`appendLog` 函数在添加新内容之前检查滚动的位置。如果聊天日志滚动到底部，则该功能将在添加内容后将新内容滚动到视图中。否则，滚动位置不会改变。

表单处理程序将用户输入写入websocket并清除输入字段。

## Docker 搭建开发调试环境

构建 `Image`：

```sh
docker build -f Dockerfile.dev -t cloud-native-game-server:dev .
```

### 启动开发环境(支持 live reload)

```sh
DEMO=2-gorilla-websocket-chat docker-compose up demo
#docker-compose down
```

进入 [localhost:3250](http://localhost:3250) 可以看到效果。


### 启动调式环境

```sh
DEMO=2-gorilla-websocket-chat docker-compose up demo-debug
```