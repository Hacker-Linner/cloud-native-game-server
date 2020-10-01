# Nano 分布式(集群)示例(Distributed Chat)

## 介绍

### 这是一个系列

1. [探索 Golang 云原生游戏服务器开发，5 分钟上手 Nano 游戏服务器框架](https://juejin.im/post/6870388583019872270)
2. [探索 Golang 云原生游戏服务器开发，根据官方示例实战 Gorilla WebSocket 的用法](https://juejin.im/post/6872641375297339399)
3. [探索 Golang 云原生游戏服务器开发，Nano 内置分布式游戏服务器方案测试用例](https://juejin.im/post/6877028133116706823)

### 示例仓库

笔者改过的官方示例：[distributed-chat](https://github.com/Hacker-Linner/cloud-native-game-server/tree/master/demo/3-distributed-chat)

### 分布式（集群）与集群的联系与区别

分布式是指将不同的业务分布在不同的地方；而集群指的是将几台服务器集中在一起，实现同一业务。

分布式中的每一个节点，都可以做集群。 而集群并不一定就是分布式的。

[分布式与集群的区别](https://kb.cnblogs.com/page/503317/)

## 探索

我们进入 `3-distributed-chat`

### 启动主服务器

用来管理或者调度集群中的其他服务器。

首先编译一下：

```sh
go build -o distributed
```

然后：
```sh
# 它的监听地址是 127.0.0.1:34567，同时也是 gRPC 服务器地址
# 它对外提供了两个服务：
# TopicService.NewUser ->> 处理来自网关的新用户请求的公共逻辑等
# TopicService.Stats ->> 集群机器服务调用统计等等
# 说白了就是 Component 里面的两个 hanlder
./distributed master --listen "127.0.0.1:34567"
```

### 启动聊天服务器并让它加入到 cluster

真正的游戏业务逻辑服务

```sh
# --master 127.0.0.1:34567 远程主服务器地址
# 它的监听地址是 127.0.0.1:34580，同时也是 gRPC 服务器地址
# 它对外提供了两个服务：
# RoomService.JoinRoom ->> 将客户端的 session 加入 Group 统一管理
# RoomService.SyncMessage ->> 广播消息，就是调用 Group 管理的 session，写信息到它们给自的 websocket 连接
./distributed chat --master "127.0.0.1:34567" --listen "127.0.0.1:34580"
```

### 启动网关服务器并让它加入到 cluster

客户端真正要连接的入口地址：

```sh
# -gate-address "127.0.0.1:34590" 这个就是客户端 websocket 连接要连接的地址
# 它的监听地址是 127.0.0.1:34570，同时也是 gRPC 服务器地址
# 它对外提供了两个服务：
# BindService.Login # 鉴权方面到处理
# BindService.BindChatServer # 直接绑定到具体到聊天服务器
./distributed gate --master "127.0.0.1:34567" --listen "127.0.0.1:34570" --gate-address "127.0.0.1:34590"
```

### 远程服务 Remote Service

集群上的每台服务器，通过 Master 节点注册后，都会把除自己以外的集群中其它节点提供的服务注册为自己的 Remote Service。

所以当我们客户端调用 `starx.notify('RoomService.SyncMessage'...`，其实网关服务器会调用它的 Remote
Service，最终会转到 `Chat Server` 节点。

### 具体流程

[http://127.0.0.1:12345/web/](http://127.0.0.1:12345/web/)

用户加入房间：`BindService.Login(Gate Server)` -> `TopicService.NewUser(Master Server)` -> `RoomService.JoinRoom(Chat Server)`

用户发送消息：`Gate Server` -> `RoomService.SyncMessage(Chat Server)`



