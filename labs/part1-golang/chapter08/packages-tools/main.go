// Package main 演示 Go 的包声明、导入与匿名导入的注册机制。
//
// 命名约定：本文件只有一个包注释与 main 入口，可测试的函数使用首字母小写的
// 未导出命名；若函数需要被其他包使用，应以首字母大写的导出命名并在注释中说明行为。
package main

import (
	"bytes"
	"errors"
	"fmt"
	"image"

	// 匿名导入：只为触发 image/png 包 init() 中的 image.RegisterFormat 注册 PNG 解码器，
	// 包本身不被直接引用。去掉本行后，image.Decode 对 PNG 数据会报 image: unknown format。
	// 同理，image/jpeg 与 image/gif 也需要各自匿名导入后才注册，image 包本身不默认注册任何格式。
	_ "image/png"
)

// tinyPNG 是一个最小合法的 1x1 像素 PNG（67 字节），以十六进制字面量内嵌在源码中，保证离线可测。
var tinyPNG = []byte{
	0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG 签名
	0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR 长度与类型
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01, // 宽 1、高 1
	0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
	0x89, 0x00, 0x00, 0x00, 0x0a, 0x49, 0x44, 0x41, // IDAT
	0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
	0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
	0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, // IEND
	0x42, 0x60, 0x82,
}

// jpegMagic 与 gifMagic 只包含格式魔数和填充字节，本身不是完整图片，
// 仅用于探测对应解码器是否已注册。
var jpegMagic = append([]byte{0xff, 0xd8, 0xff, 0xe0}, make([]byte, 64)...)

var gifMagic = append([]byte("GIF89a"), make([]byte, 64)...)

// decodeFormat 用 image.Decode 解码 data 并返回格式名（如 "png"）。
// image.Decode 只能识别已注册解码器的格式；未注册时报 image: unknown format（image.ErrFormat）。
func decodeFormat(data []byte) (string, error) {
	_, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	return format, nil
}

// formatRegistered 用魔数探测某种格式的解码器是否已注册：
// 未注册时错误为 image.ErrFormat；已注册时要么解码成功，要么因数据不完整返回格式专属的解码错误。
func formatRegistered(data []byte) bool {
	_, _, err := image.DecodeConfig(bytes.NewReader(data))
	return !errors.Is(err, image.ErrFormat)
}

// registeredFormats 用各格式的魔数探测当前已注册的解码器，返回可识别的格式名列表。
// 本例只匿名导入了 image/png，因此结果是 ["png"]；
// 再添加 import _ "image/jpeg" 或 import _ "image/gif" 后，对应格式才会出现在列表中。
func registeredFormats() []string {
	probes := []struct {
		name string
		data []byte
	}{
		{"png", tinyPNG},
		{"jpeg", jpegMagic},
		{"gif", gifMagic},
	}
	formats := make([]string, 0, len(probes))
	for _, p := range probes {
		if formatRegistered(p.data) {
			formats = append(formats, p.name)
		}
	}
	return formats
}

func main() {
	format, err := decodeFormat(tinyPNG)
	if err != nil {
		fmt.Println("decode error:", err)
		return
	}
	fmt.Println("内嵌图片格式:", format)

	img, _, err := image.Decode(bytes.NewReader(tinyPNG))
	if err != nil {
		fmt.Println("decode error:", err)
		return
	}
	fmt.Printf("图片尺寸: %d x %d\n", img.Bounds().Dx(), img.Bounds().Dy())

	fmt.Println("已注册格式:", registeredFormats())
}
