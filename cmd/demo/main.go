// Command demo builds the local-only SQLite documentation environment.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
)

const (
	defaultDBPath    = ".local/message-push-demo.sqlite"
	defaultRedisAddr = "127.0.0.1:6379"
	defaultRedisDB   = 15
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "demo:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 || args[0] != "reset" {
		return fmt.Errorf("用法: go run ./cmd/demo reset [--db %s] [--redis-addr %s] [--redis-db %d]", defaultDBPath, defaultRedisAddr, defaultRedisDB)
	}

	flags := flag.NewFlagSet("reset", flag.ContinueOnError)
	dbPath := flags.String("db", defaultDBPath, "SQLite 演示数据库文件")
	redisAddr := flags.String("redis-addr", defaultRedisAddr, "Redis 地址")
	redisDB := flags.Int("redis-db", defaultRedisDB, "Redis DB（必须为 15）")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("无法识别的参数: %v", flags.Args())
	}
	if *redisDB != defaultRedisDB {
		return fmt.Errorf("为避免清理其他环境，演示命令仅允许 Redis DB %d", defaultRedisDB)
	}

	summary, err := resetDemo(ctx, resetOptions{
		DBPath:    *dbPath,
		RedisAddr: *redisAddr,
		RedisDB:   *redisDB,
	})
	if err != nil {
		return err
	}

	fmt.Printf("SQLite 演示库已重建: %s\n", summary.DBPath)
	fmt.Printf("假数据: %d 个应用，%d 个通道，%d 条发送任务，%d 条上行短信\n", summary.Applications, summary.Channels, summary.Tasks, summary.UpstreamMessages)
	fmt.Println("本地账号: demo-admin / demo-pass-2026（仅限本机演示）")
	return nil
}
