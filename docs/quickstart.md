# 5 分钟上手 Nano 游戏服务器框架

## 介绍

### Nano 是什么？

轻量级，方便，高性能 `golang` 的游戏服务器框架。

`nano` 是一个轻量级的服务器框架，它最适合的应用领域是网页游戏、社交游戏、移动游戏的服务端。当然还不仅仅是游戏，用 `nano` 开发高实时 `web` 应用也非常合适。

**最重要的是可以通过这个入门 Golang 游戏服务器框架开发**

### 示例仓库

[cloud-native-game-server](https://github.com/Hacker-Linner/cloud-native-game-server)

## 使用 Nano 快速搭建一个 Chat Room

### 一句话描述 Nano 术语

* 组件(`Component`)：`nano` 应用的功能就是由一些松散耦合的 `Component` 组成的，每个 `Component` 完成一些功能。
* `Handler`：它定义在 `Component` 内的方法，用来处理具体的业务逻辑。
* 路由(`Route`)：用来标识一个`具体服务` 或者客户端接受服务端推送消息的`位置`。
* 会话(`Session`)：客户端连接服务器后, 建立一个会话保存连接期间一些上下文信息。连接断开后释放。
* 组(`Group`)：`Group` 可以看作是一个 `Session` 的容器，主要用于需要广播推送消息的场景。
* 请求(`Request`), 响应(`Response`), 通知(`Notify`), 推送(`Push`)：`Nano` 中四种消息类型。

### 组件的生命周期

```sh
type DemoComponent struct{}

func (c *DemoComponent) Init()           {}
func (c *DemoComponent) AfterInit()      {}
func (c *DemoComponent) BeforeShutdown() {}
func (c *DemoComponent) Shutdown()       {}
```

* Init：组件初始化时将被调用。
* AfterInit：组件初始化完成后将被调用。
* BeforeShutdown：组件销毁之前将被调用。
* Shutdown：组件销毁时将被调用。

整个组件的生命周期看起来非常的清晰。

### 一句话描述业务

* 用户可以加入具体房间
* 用户可以看到房间内所有成员
* 用户可以在当前房间发送消息

### 业务具体分析

* 用户可以加入具体房间
  * 请求加入(`Request`) -> `Request` 对应 `nano` 一种消息类型
  * 需要响应(`Response`)是否允许加入 -> `Response` 对应 `nano` 一种消息类型
* 用户可以看到房间内所有成员
  * 服务端主动推送(`Push`)房间内所有成员`Members` -> `Push` 对应 `nano` 一种消息类型
  * 服务端主动广播📢(`Push`)房间内其它成员，有新人加入`New user`
* 用户可以在当前房间发送消息
  * 用户发送(`Notify`)消息到当前房间 -> `Notify` 对应 `nano` 一种消息类型，不需要服务器对他有所回应
  * 服务器将消息📢(`Push`)给房间其它成员

至此，我们了解了业务，然后通过业务我们又了解了 `Nano` 的四种消息类型应用。

## Demo 源码解析

`demo/1-nano-chat`

```go
type (
  // 房间的定义
	Room struct {
    // 管理房间内所有的会话
		group *nano.Group
	}

  // RoomManager 表示一个包含一堆房间的组件，他是 nano 组件，可在生命周期内 hook 逻辑
	RoomManager struct {
    // 继承 nano 组件，拥有完整的生命周期
    component.Base
    // 组件初始化完成后，做一些定时任务
    timer *scheduler.Timer
    // 多个房间，key-value 存储
		rooms map[int]*Room
	}

  // 表示一个用户发送的消息定义
	UserMessage struct {
		Name    string `json:"name"`
		Content string `json:"content"`
	}

  // 当新用户加入房间时将收到新用户消息（广播）
	NewUser struct {
		Content string `json:"content"`
	}

	// 包含所有成员的 UID 
	AllMembers struct {
		Members []int64 `json:"members"`
	}

	// 表示加入房间服务端的响应结果
	JoinResponse struct {
		Code   int    `json:"code"`
		Result string `json:"result"`
	}

  // 流量统计
	Stats struct {
    // 继承 nano 组件，拥有完整的生命周期
    component.Base
    // 组件初始化完成后，做一些定时任务
    timer         *scheduler.Timer
    // 出口流量统计
    outboundBytes int
    // 入口流量统计
		inboundBytes  int
	}
)

// 统计出口流量，会定义到 nano 的 pipeline
func (stats *Stats) outbound(s *session.Session, msg *pipeline.Message) error {
	stats.outboundBytes += len(msg.Data)
	return nil
}

// 统计入口流量，会定义到 nano 的 pipeline
func (stats *Stats) inbound(s *session.Session, msg *pipeline.Message) error {
	stats.inboundBytes += len(msg.Data)
	return nil
}

// 组件初始化完成后，会调用
// 每分钟会打印下出口与入口的流量
func (stats *Stats) AfterInit() {
	stats.timer = scheduler.NewTimer(time.Minute, func() {
		println("OutboundBytes", stats.outboundBytes)
		println("InboundBytes", stats.outboundBytes)
	})
}

func (st *Stats) Nil(s *session.Session, msg []byte) error {
	return nil
}

const (
  // 测试房间 id
  testRoomID = 1
  // 测试房间 key
	roomIDKey  = "ROOM_ID"
)

// 初始化 RoomManager
func NewRoomManager() *RoomManager {
	return &RoomManager{
		rooms: map[int]*Room{},
	}
}

// RoomManager 初始化完成后将被调用
func (mgr *RoomManager) AfterInit() {
  // 用户断开连接后将会被调用
  // 将它从房间中移除
	session.Lifetime.OnClosed(func(s *session.Session) {
		if !s.HasKey(roomIDKey) {
			return
		}
    room := s.Value(roomIDKey).(*Room)
    // 移除这个会话
		room.group.Leave(s)
  })
  
  // 一个定时任务，每分钟打印下房间的成员数量
	mgr.timer = scheduler.NewTimer(time.Minute, func() {
		for roomId, room := range mgr.rooms {
			println(fmt.Sprintf("UserCount: RoomID=%d, Time=%s, Count=%d",
				roomId, time.Now().String(), room.group.Count()))
		}
	})
}

// 加入房间的业务逻辑处理
func (mgr *RoomManager) Join(s *session.Session, msg []byte) error {
	// 注意：这里 demo 仅仅只是加入 testRoomID
	room, found := mgr.rooms[testRoomID]
	if !found {
		room = &Room{
			group: nano.NewGroup(fmt.Sprintf("room-%d", testRoomID)),
		}
		mgr.rooms[testRoomID] = room
	}

	fakeUID := s.ID() // 这里仅仅是用 sessionId 模拟下 uid
	s.Bind(fakeUID)   // 绑定 uid 到 session
  s.Set(roomIDKey, room) // 设置一下当前 session 关联到的房间
  // 推送房间所有成员到当前的 session
  s.Push("onMembers", &AllMembers{Members: room.group.Members()})
	// 广播房间内其它成员，有新人加入
  room.group.Broadcast("onNewUser", &NewUser{Content: fmt.Sprintf("New user: %d", s.ID())})
	// 将 session 加入到房间 group 统一管理
  room.group.Add(s)
  // 回应当前用户加入成功
	return s.Response(&JoinResponse{Result: "success"})
}

// 同步最新的消息给房间内所有成员
func (mgr *RoomManager) Message(s *session.Session, msg *UserMessage) error {
	if !s.HasKey(roomIDKey) {
		return fmt.Errorf("not join room yet")
	}
  room := s.Value(roomIDKey).(*Room)
  // 广播
	return room.group.Broadcast("onMessage", msg)
}

func main() {
  // 新建组件容器实例
  components := &component.Components{}
  // 注册组件
	components.Register(
    // 组件实例
    NewRoomManager(),
    // 重写组件名字
    component.WithName("room"),
    // 重写组件 handler 名字，这里是小写
		component.WithNameFunc(strings.ToLower),
	)
	// 流量统计
	pip := pipeline.New()
  var stats = &stats{}
  // 入队 Outbound pipeline 
  pip.Outbound().PushBack(stats.outbound)
  // 入队 Inbound pipeline
	pip.Inbound().PushBack(stats.inbound)
	// 注册下流量统计组件
	components.Register(stats, component.WithName("stats"))
  // 设置日志打印格式
  log.SetFlags(log.LstdFlags | log.Llongfile)
  // web 静态资源处理
	http.Handle("/web/", http.StripPrefix("/web/", http.FileServer(http.Dir("web"))))
  // 启动 nano 
	nano.Listen(":3250", // 端口号
		nano.WithIsWebsocket(true), // 是否使用 websocket
		nano.WithPipeline(pip), // 是否使用 pipeline
		nano.WithCheckOriginFunc(func(_ *http.Request) bool { return true }), // 允许跨域
		nano.WithWSPath("/nano"), // websocket 连接地址
		nano.WithDebugMode(),  // 开启 debug 模式
		nano.WithSerializer(json.NewSerializer()), // 使用 json 序列化器
		nano.WithComponents(components), // 加载组件
	)
}
```

前端代码非常简单，大家直接看 [cloud-native-game-server](https://github.com/Hacker-Linner/cloud-native-game-server)

## Docker 搭建开发调试环境

### Dockerfile

`Dockerfile.dev`

```yaml
FROM golang:1.14

WORKDIR /workspace

# 阿里云
RUN go env -w GO111MODULE=on
RUN go env -w GOPROXY=https://mirrors.aliyun.com/goproxy/,direct

# debug
RUN go get github.com/go-delve/delve/cmd/dlv

# live reload
RUN go get -u github.com/cosmtrek/air

# nano
RUN go mod init cloud-native-game-server
RUN go get github.com/lonng/nano@master
```

构建 `Image`：

```sh
docker build -f Dockerfile.dev -t cloud-native-game-server:dev .
```

### docker-compose.yaml

```yaml
version: "3.4"
services:

  demo:
    image: cloud-native-game-server:dev
    command: >
      bash -c "cp ./go.mod ./go.sum app/
      && cd app/demo/${DEMO}
      && ls -la
      && air -c ../../.air.toml -d"
    volumes:
    - ./:/workspace/app
    ports:
      - 3250:3250
  
  demo-debug:
    image: cloud-native-game-server:dev
    command: >
      bash -c "cp ./go.mod ./go.sum app/
      && cd app/demo/${DEMO}
      && ls -la
      && dlv debug main.go --headless --log -l 0.0.0.0:2345 --api-version=2"
    volumes:
    - ./:/workspace/app
    ports:
      - 3250:3250
      - 2345:2345
    security_opt:
      - "seccomp:unconfined"
```

### 启动开发环境(支持 live reload)

```sh
# 如我要开发 1-nano-chat
DEMO=1-nano-chat docker-compose up demo
```

进入 [localhost:3250/web/](http://localhost:3250/web/) 可以看到效果。


### 启动调式环境

```sh
# 如我要调试 1-nano-chat
DEMO=1-nano-chat docker-compose up demo-debug
```

## 参考

* [官方 Github](https://github.com/lonng/nano)
* [官方教程 — 如何构建你的第一个nano应用](https://github.com/lonng/nano/blob/master/docs/get_started_zh_CN.md)
* [官方 Demo-starx-chat-demo](https://github.com/lonng/nano/tree/master/examples/demo/chat)
