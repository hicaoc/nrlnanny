NRL互联 保姆程序说明

## 功能说明

### 1.1 群组监听
程序能够监听指定的群组，并记录相关的通信数据。

### 1.2 录音
程序可以录制接收到的音频数据，并保存到指定的文件路径。

### 1.3 信标定时播放
程序可以根据配置的定时任务，定期播放预设的信标文件。

### 1.4 信标按文件名时间点播放
程序可以识别文件名中的时间点，并按时间点播放信标文件。

### 1.5 音频轮播
程序可以按文件名末尾的顺序号播放指定文件夹下的音频文件。支持内嵌解码 WAV、MP3、FLAC、AAC/ADTS，以及 M4A/MP4 容器中的 AAC-LC，不调用外部解码程序。

### 1.6 网络电台
控制台可以收藏多个“电台名称 + 网络地址”，并将网络流实时转发到 NRL 服务器。支持 MP3 直链和承载 AAC-LC 音频的 M3U8/HLS（MPEG-TS、fMP4、低延迟 HLS）。网络流、HLS 分片和音频均由程序内嵌的纯 Go 组件解析与解码，不调用 `ffmpeg` 或其他外部程序。播放网络电台时会暂停本地音乐，避免两个节目同时发送。

### 1.7 麦克风通话
程序可以通过采集电脑麦克风，其他音频输入设备，并将音频发送给NRL互联网络。
Windows使用免费的 https://vb-audio.com/Cable/index.htm 虚拟声卡驱动。可以转接第三方软件，如QQ音乐，Foobar2000等任何软件音频

## 前提条件

### 1.1 音频文件准备
准备8000Hz采样率、单声道、16位深度的WAV文件。如果格式不对，可以按需使用 FFmpeg 等工具在导入前转换（仅用于离线准备，本程序运行和网络电台解码不依赖 FFmpeg）：
```bash
ffmpeg -i test1.wav -ac 1 -ar 8000 test3.wav
```

### 1.2 安装音频支持库
在Linux系统上，需要安装音频支持库：
```bash
sudo apt install libasound2-dev
```

find . -type f -size -47000c -delete

### 1.3 配置文件修改
编辑配置文件 `nrlnanny.yaml`，根据需要修改以下参数：

- **Server**: 服务器地址，例如 `"js.nrlptt.com"`
- **Port**: 连接端口，例如 `"60050"`
- **Callsign**: 虚拟盒子的所有者呼号，例如 `"BH4RPN"`
- **SSID**: 虚拟盒子SSID（目前不支持修改）， 内置`250`
- **Volume**: 麦克风通话的音量，例如 `0.5`
- **SendOpus**: 发送编码开关；`true` 使用16 kHz Opus/type 8，`false` 使用8 kHz G.711 A-law/type 1
- **music_file_Path**: 音乐文件路径，例如 `"./music"`
- **AudioFile**: 信标文件路径和文件名，如果为空则不播放信标，例如 `"./test.wav"`
- **RecoderFilePath**: WAV录音保存路径，例如 `"./recoder"`

信标、定时播放和音乐轮播均支持 WAV、MP3、FLAC、AAC/ADTS 和 M4A/MP4（AAC-LC）。音频会自动混合为单声道并重采样到 16 kHz；发送 G711 时再降采样到 8 kHz。
- **CronString**: CRON格式的定时配置，默认是每10分钟一次，例如 `"*/10 * * * *"`
- **WebPort**: 网页监听端口，例如 `"8080"`
- 多房间直播 WebSocket 会自动使用当前 **Server**，例如 `Server: "example.com"` 对应 `wss://example.com/ws/calls`
- **EnableControlPage**: 是否允许登录控制台；首页始终为 Live，设为 `false` 时完全关闭控制台登录
- **ControlUsername / ControlPassword**: 控制台登录用户名和密码；两项必须同时配置
- 录音浏览和录音文件公开访问；登录成功后才显示控制台导航。会话有效期为 12 小时
- **RadioStations**: 收藏的网络电台列表，可通过控制台维护
- **RadioActiveID**: 当前选择的电台 ID
- **RadioPlaying**: 程序启动时是否恢复播放网络电台

### AT 指令

- `AT+OPUS=ON|OFF|?`：开启、关闭或查询 Opus 发送；兼容 `AT+SEND_OPUS`
- `AT+RADIO_LIST=1`：查询收藏电台，回包包含电台 ID 和名称
- `AT+RADIO_STATUS=?`：查询当前电台状态
- `AT+RADIO_PLAY=<ID>`：播放指定电台
- `AT+RADIO_STOP=1`：停止网络电台
- `AT+RADIO_ADD=<名称>,<URL>`：收藏电台（URL 支持包含 `=`）
- `AT+RADIO_DELETE=<ID>`：删除收藏电台

## 安装步骤

### 1. 克隆仓库
```bash
git clone https://github.com/hicaoc/nrlnanny.git
cd nrlnanny
```

### 2. 安装依赖
```bash
make install
```

### 3. 编译程序
```bash
make build
```

## 使用示例

### 1. 启动程序
```bash
./nrlnanny
```

### 2. 查看日志
```bash
tail -f nrlnanny.log
```

## 依赖

- Go语言环境
- libasound2-dev

## 故障排除

### 1. 音频问题
确保音频设备已正确连接，并且系统音频设置正确。

### 2. 网络问题
检查网络连接是否正常，确保能够访问配置中的服务器地址和端口。

### 3. 权限问题
确保程序有足够的权限访问录音保存路径和配置文件。
