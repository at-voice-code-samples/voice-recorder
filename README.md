##### As simple voice recording app

###### Build:
```
go get -u github.com/go-redis/redis
go build app.go
```

###### Run:
```
./app <callback-url> <port>
```

###### Example:
```
./app 'http://app.ngrok.io' 8080
```
