
```sh

go build -o distributed

# run master server
./distributed master
./distributed chat --listen "127.0.0.1:34580"
./distributed gate --listen "127.0.0.1:34570" --gate-address "127.0.0.1:34590"
```

