package main

import (
	"fmt"

	"github.com/eiannone/keyboard"
)

func keyboardrun() {
	// 初始化键盘监听（进入 raw 模式）
	err := keyboard.Open()
	if err != nil {
		panic(err)
	}
	defer keyboard.Close()

	fmt.Println("按 ↑ 或 ↓ 键 调节音量，← 重新播放，→ 下一首，空格 暂停/播放")

	for {
		_, key, err := keyboard.GetKey()
		if err != nil {
			//panic(err)
			fmt.Println(err)
		}

		fmt.Println()

		// 处理方向键
		switch key {
		case keyboard.KeyArrowUp:

			if conf.System.Volume < 2 {
				conf.System.Volume = conf.System.Volume + 0.01
			}

			fmt.Printf("你按了 ↑ 上键,音量增加到：%d%%\n", int(conf.System.Volume*100))
		case keyboard.KeyArrowDown:

			if conf.System.Volume > 0.01 {
				conf.System.Volume = conf.System.Volume - 0.01
			}

			fmt.Printf("你按了 ↓ 下键,音量减少到：%d%%\n", int(conf.System.Volume*100))
		case keyboard.KeyArrowLeft:
			lastmusic <- true
			fmt.Println("你按了 ← 左键,重新播放")
		case keyboard.KeyArrowRight:
			nextmusic <- true
			fmt.Println("你按了 → 右键，下一首")

		case keyboard.KeySpace:
			pausemusic <- true
			fmt.Println("你按了 空格键，暂停/播放")
		default:
			// 可选：忽略其他按键
		}
	}
}
