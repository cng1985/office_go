package config

import (
	"flag"
	"fmt"
	"io"
)

type flagValues struct {
	baseDir   *string
	configDir *string
	dataDir   *string
	profile   *string
	fs        *flag.FlagSet
}

func newFlagSet() *flagValues {
	fs := flag.NewFlagSet("yunjian", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	values := &flagValues{fs: fs}
	values.baseDir = fs.String("base-dir", "", "应用根目录")
	values.configDir = fs.String("config-dir", "", "配置目录")
	values.dataDir = fs.String("data-dir", "", "数据目录")
	values.profile = fs.String("profile", "", "环境配置：dev 或 pro")
	fs.Usage = func() {}
	return values
}

func (f *flagValues) Parse(args []string) error {
	if err := f.fs.Parse(args); err != nil {
		return fmt.Errorf("解析命令行失败: %w", err)
	}
	return nil
}

func (f *flagValues) NArg() int { return f.fs.NArg() }

func (f *flagValues) Arg(index int) string { return f.fs.Arg(index) }
