NRL互联 保姆程序说明

## 功能说明

### 1.1 群组监听
程序能够监听指定的群组，并记录相关的通信数据。

### 1.2 录音
程序可以录制接收到的音频数据，并保存到指定的文件路径。

### 1.3 信标定时播放
程序可以根据配置的定时任务，定期播放预设的信标文件。

## 前提条件

### 1.1 音频文件准备
准备8000Hz采样率、单声道、16位深度的WAV文件。如果格式不对，可以使用以下命令进行转换：
```bash
ffmpeg -i test1.wav -ac 1 -ar 8000 test3.wav
```

### 1.2 安装音频支持库
在Linux系统上，需要安装音频支持库：
```bash
sudo apt install libasound2-dev
```

### 1.3 配置文件修改
编辑配置文件 `nrlnanny.yaml`，根据需要修改以下参数：

- **Server**: 服务器地址，例如 `"js.nrlptt.com"`
- **Port**: 连接端口，例如 `"60050"`
- **Callsign**: 虚拟盒子的所有者呼号，例如 `"BH4RPN"`
- **SSID**: 虚拟盒子SSID（目前不支持修改）， 内置`250`
- **AudioFile**: 信标文件路径和文件名，如果为空则不播放信标，例如 `"./test.wav"`
- **RecoderFilePath**: WAV录音保存路径，例如 `"./recoder"`
- **CronString**: CRON格式的定时配置，默认是每10分钟一次，例如 `"*/10 * * * *"`

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
- FFmpeg
- libasound2-dev

## 故障排除

### 1. 音频问题
确保音频设备已正确连接，并且系统音频设置正确。

### 2. 网络问题
检查网络连接是否正常，确保能够访问配置中的服务器地址和端口。

### 3. 权限问题
确保程序有足够的权限访问录音保存路径和配置文件。
