本练习代码用golang的websocket和http服务，提供聊天室相关支持，没有处理相关安全测试和检测，没有做数据持久化，仅练习用

1.运行server.exe启动服务器
  服务器用的端口号为8081，禁用则需放开<br>
2.直接网页打开login.html即可<br>
3.服务器代码为server.go, 本地有go环境, 可调整相关代码, go run 源文件重启<br>
  如需调整server.go代码需要下载websocket库<br>
  步骤:<br>
  go get -u github.com/golang/net/websocket<br>
  完了之后把websocket复制到golang.org/x/net/websocket即可使用<br>

简单的练习代码，勿喷！！
