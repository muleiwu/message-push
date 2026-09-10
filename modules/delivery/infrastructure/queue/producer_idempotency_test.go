package queue

import (
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"cnb.cool/mliev/push/message-push/app/model"
	"github.com/redis/go-redis/v9"
)

func TestPushDelayedOnceDoesNotReplayConsumedEffect(t *testing.T) {
	binary, err := exec.LookPath("redis-server")
	if err != nil {
		t.Skip("redis-server unavailable; isolated Lua integration test requires it")
	}
	dir, err := os.MkdirTemp("", "sms-redis-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	// A relative socket keeps macOS's Unix-socket path under its length limit.
	command := exec.Command(binary, "--port", "0", "--unixsocket", "redis.sock", "--save", "", "--appendonly", "no")
	command.Dir, command.Stdout, command.Stderr = dir, io.Discard, io.Discard
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	client := redis.NewClient(&redis.Options{Network: "unix", Addr: filepath.Join(dir, "redis.sock"), MaxRetries: 0})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	for client.Ping(ctx).Err() != nil {
		if ctx.Err() != nil {
			t.Fatal("isolated Redis did not start")
		}
		time.Sleep(10 * time.Millisecond)
	}
	producer := NewProducer(client)
	task := &model.PushTask{TaskID: "fixture-task"}
	at := time.Now()
	if err := producer.PushDelayedOnce(ctx, task, at, "fixture-event"); err != nil {
		t.Fatal(err)
	}
	if n := client.ZCard(ctx, "push:scheduled:tasks").Val(); n != 1 {
		t.Fatalf("queued %d tasks", n)
	}
	if err := client.ZRem(ctx, "push:scheduled:tasks", task.TaskID).Err(); err != nil {
		t.Fatal(err)
	}
	// Simulate restart after Redis accepted the effect but before the inbox ack.
	if err := NewProducer(client).PushDelayedOnce(ctx, task, at, "fixture-event"); err != nil {
		t.Fatal(err)
	}
	if n := client.ZCard(ctx, "push:scheduled:tasks").Val(); n != 0 {
		t.Fatal("consumed effect was enqueued again")
	}
	if err := producer.PushDelayedOnce(ctx, task, at, "next-event"); err != nil {
		t.Fatal(err)
	}
	if n := client.ZCard(ctx, "push:scheduled:tasks").Val(); n != 1 {
		t.Fatal("new event was suppressed")
	}
}
