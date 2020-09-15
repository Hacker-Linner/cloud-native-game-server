// Copyright 2013 The Gorilla WebSocket Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

// Hub 维护一组活动客户端并向客户端广播消息。
type Hub struct {
	// 已注册的 Client
	// (维护当前 hub 所有活跃的 websocket)
	clients map[*Client]bool

	// 来自 Client 的入站消息
	// (广播的管道)
	broadcast chan []byte

	// 注册来自客户端的请求
	// (注册 Client 的管道)
	register chan *Client

	// 取消注册来自客户端的请求。
	// (注销 Client 的管道)
	unregister chan *Client
}

func newHub() *Hub {
	// 返回一个 Hub 实例
	return &Hub{
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		clients:    make(map[*Client]bool),
	}
}

func (h *Hub) run() {
	for {
		select {
		// 注册
		case client := <-h.register:
			h.clients[client] = true
		// 注销
		case client := <-h.unregister:
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
		// 广播
		case message := <-h.broadcast:
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
		}
	}
}
