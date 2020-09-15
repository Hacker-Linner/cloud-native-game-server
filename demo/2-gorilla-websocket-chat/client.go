// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"bytes"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// 允许向 peer 写入消息的时间。
	// (当前客户端 websocket 写入消息的超时时间)
	writeWait = 10 * time.Second

	// 允许从 peer 读取下一个 pong 消息的时间。
	// (当前从客户端 websocket 读入消息的最大间隔时间，也就是你1分钟不理服务器，服务器从此不再理你)
	pongWait = 60 * time.Second

	// 使用 period 向 peer 发送 ping。要比 pongWait 小。
	// (一个 ticker, 就是说 -> 每隔多少时间检测下当前客户端 websocket 的状态)
	pingPeriod = (pongWait * 9) / 10

	// peer 允许的最大消息大小。
	// (从当前客户端 websocket 读取消息最大大小)
	maxMessageSize = 512
)

var (
	newline = []byte{'\n'}
	space   = []byte{' '}
)

// Upgrader 指定了将 HTTP 连接升级为 WebSocket 连接的参数。
// 设置适当的读写缓冲区，提升性能
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

//Client 是 websocket 连接和 hub 之间的中间人。
type Client struct {
	// 管理所有的客户端
	hub *Hub

	// websocket 连接
	conn *websocket.Conn

	// 出站消息的缓冲通道。
	send chan []byte
}

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

// serveWs 处理来自 peer 的websocket请求。
func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	client := &Client{hub: hub, conn: conn, send: make(chan []byte, 256)}
	client.hub.register <- client

	// 通过在新的goroutines中完成所有工作，允许调用者引用内存的集合。
	go client.writePump()
	go client.readPump()
}
