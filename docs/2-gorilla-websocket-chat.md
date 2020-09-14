# 从 0 到 1 跟随官方示例实战 Gorilla WebSocket 使用

## 介绍

### 这是一个系列

1. [探索 Golang 云原生游戏服务器开发，5 分钟上手 Nano 游戏服务器框架](https://juejin.im/post/6870388583019872270)

### Gorilla WebSocket 是什么？

Gorilla WebSocket 是 WebSocket 协议的 Go 实现。

WebSocket 是啥？为少这里就不赘述了，掘友们在[掘金](https://juejin.im/search?query=websocket&type=all)上科普了太多太多😂。

### 示例仓库
* 官方例子：[Chat example](https://github.com/gorilla/websocket/tree/master/examples/chat)
* 为上更改过的例子：[cloud-native-game-server/2-gorilla-websocket-chat](https://github.com/Hacker-Linner/cloud-native-game-server/tree/master/demo/2-gorilla-websocket-chat)

## 示例分析

这里我整理下这个例子的官方 [README.md](https://github.com/gorilla/websocket/tree/master/examples/chat)

### Server

服务器应用程序定义两种类型，`Client` 和 `Hub`。服务器为每个 websocket 连接创建一个 `Client` 类型的实例。
`Client` 充当 websocket 连接和 `Hub` 类型的单个实例之间的中介。`Hub` 维护一组注册的客户端，并向客户端广播消息。

应用程序为 `Hub` 运行一个 goroutine，为每个 `Client` 运行两个 goroutine。多个 goroutine 使用通道相互通信。该 `Hub` 有用于注册客户端、取消注册客户端和广播消息的通道。`Client` 有一个缓冲的出站消息通道。客户端的 goroutine 之一从该通道读取消息，并将消息写入 websocket。另一个客户端 goroutine 从 websocket 读取消息并将其发送到 hub。

### Hub 



