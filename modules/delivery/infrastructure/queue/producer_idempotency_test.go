package queue

import (
	"context"
	"io"
	"net"
	"os/exec"
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
	// Loopback TCP keeps the isolated server portable: Windows builds of
	// redis-server reject --unixsocket, which the original Unix-socket setup
	// relied on.
	addr := freeLoopbackAddr(t)
	command := exec.Command(binary, "--port", addr[1], "--bind", addr[0], "--save", "", "--appendonly", "no")
	command.Stdout, command.Stderr = io.Discard, io.Discard
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = command.Process.Kill(); _ = command.Wait() }()
	client := redis.NewClient(&redis.Options{Network: "tcp", Addr: addr[0] + ":" + addr[1], MaxRetries: 0})
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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

// freeLoopbackAddr reserves an unused loopback TCP port for the duration of the
// test binary and returns the host and port to hand to redis-server.
func freeLoopbackAddr(t *testing.T) [2]string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve loopback port: %v", err)
	}
	defer listener.Close()
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split loopback addr: %v", err)
	}
	return [2]string{host, port}
}

