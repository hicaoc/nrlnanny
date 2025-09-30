# 镜像功能
NRL互联服务器的保姆程序， 用于对房间录音，监听，定时发送信标等功能。


# 下载镜像
```docker pull  hicaoc/nrlnanny:latest```

# 启动镜像
  * 映射本地目录，持久化配置文件和数据文件


```docker run -d  -p 80:80 -p 60050:60050/udp -v /hostdir/data:/nrlnanny/data -v /hostdir/conf:/nrlnanny/conf  hicaoc/nrlnanny:latest```