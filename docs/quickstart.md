# 5 分钟上手 Nano 游戏服务器

## 介绍

* [官方 Github](https://github.com/lonng/nano)
* [官方教程 — 如何构建你的第一个nano应用](https://github.com/lonng/nano/blob/master/docs/get_started_zh_CN.md)：笔者将基于这个教程并采用 Docker 的方式快速搭建本地开发调试环境。

## Docker 搭建开发调试环境

```sh
docker build -f Dockerfile.dev -t cloud-native-game-server:dev .

docker-compose down
docker-compose up 1-nano-websocket

DEMO=1-nano-websocket docker-compose up demo
```

