package infrastructure

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	domain "cnb.cool/mliev/push/message-push/modules/sender/domain"
)

func TestNormalizeEmailSubject(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{name: "reject empty", input: "", wantErr: true},
		{name: "reject whitespace", input: "  \t ", wantErr: true},
		{name: "trim normal", input: "  订单通知  ", want: "订单通知"},
		{name: "reject CRLF injection", input: "正常主题\r\nBcc: attacker@example.com", wantErr: true},
		{name: "reject bare carriage return", input: "主题\rBcc: attacker@example.com", wantErr: true},
		{name: "reject bare newline", input: "主题\nBcc: attacker@example.com", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := normalizeEmailSubject(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("normalizeEmailSubject(%q) = %q, want error", tt.input, got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("normalizeEmailSubject(%q) = %q, %v; want %q", tt.input, got, err, tt.want)
			}
		})
	}
}

func TestSMTPProviderRequiresTitleMapping(t *testing.T) {
	meta, err := domain.GetByCode(constants.ProviderSMTP)
	if err != nil {
		t.Fatalf("GetByCode(%q) error = %v", constants.ProviderSMTP, err)
	}
	if !meta.RequiresSignature {
		t.Fatal("SMTP provider must require a mapped title before sending")
	}
}

func TestSMTPSendUsesMappedTitleInsteadOfTaskAlias(t *testing.T) {
	host, port, messages := startSMTPRecorder(t, 1)
	sender := NewSMTPSender()
	response, err := sender.Send(context.Background(), &domain.SendRequest{
		Task: &model.PushTask{
			TaskID:    "email-1",
			Receiver:  "recipient@example.com",
			Signature: "order-created",
		},
		ProviderAccount: &model.ProviderAccount{Config: fmt.Sprintf(
			`{"host":%q,"port":%d,"username":"user","password":"pass","from":"sender@example.com","encryption":"none"}`,
			host,
			port,
		)},
		Signature:       &model.ProviderSignature{SignatureCode: "订单已经创建"},
		RenderedContent: "邮件正文",
	})
	if err != nil {
		t.Fatalf("Send() error = %v", err)
	}
	if response == nil || !response.Success {
		t.Fatalf("Send() response = %+v, want success", response)
	}
	if !strings.Contains(response.RequestData, `"subject":"订单已经创建"`) {
		t.Fatalf("Send() request audit data = %s, want mapped subject", response.RequestData)
	}

	message := <-messages
	if !strings.Contains(message, "Subject: 订单已经创建\r\n") {
		t.Fatalf("SMTP message did not use mapped title:\n%s", message)
	}
	if strings.Contains(message, "Subject: order-created\r\n") {
		t.Fatalf("SMTP message used task alias as title:\n%s", message)
	}
}

func TestSMTPSendRejectsMissingOrEmptyMappedTitle(t *testing.T) {
	tests := []struct {
		name      string
		signature *model.ProviderSignature
	}{
		{name: "missing mapping"},
		{name: "empty mapped title", signature: &model.ProviderSignature{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := NewSMTPSender().Send(context.Background(), &domain.SendRequest{
				Task:            &model.PushTask{TaskID: "email-invalid", Receiver: "recipient@example.com", Signature: "alias"},
				ProviderAccount: &model.ProviderAccount{Config: `{}`},
				Signature:       tt.signature,
				RenderedContent: "邮件正文",
			})
			if err == nil || !strings.Contains(err.Error(), "email title") {
				t.Fatalf("Send() response = %+v, error = %v; want mapped title error", response, err)
			}
		})
	}
}

func TestSMTPBatchSendUsesMappedTitleInsteadOfTaskAlias(t *testing.T) {
	host, port, messages := startSMTPRecorder(t, 1)
	response, err := NewSMTPSender().BatchSend(context.Background(), &domain.BatchSendRequest{
		Tasks: []*model.PushTask{{
			TaskID:    "email-batch-1",
			Receiver:  "recipient@example.com",
			Signature: "order-created",
		}},
		ProviderAccount: &model.ProviderAccount{Config: fmt.Sprintf(
			`{"host":%q,"port":%d,"username":"user","password":"pass","from":"sender@example.com","encryption":"none"}`,
			host,
			port,
		)},
		Signature:       &model.ProviderSignature{SignatureCode: "批量订单通知"},
		RenderedContent: "邮件正文",
	})
	if err != nil {
		t.Fatalf("BatchSend() error = %v", err)
	}
	if response == nil || len(response.Results) != 1 || !response.Results[0].Success {
		t.Fatalf("BatchSend() response = %+v, want one success", response)
	}
	if !strings.Contains(response.Results[0].RequestData, `"subject":"批量订单通知"`) {
		t.Fatalf("BatchSend() request audit data = %s, want mapped subject", response.Results[0].RequestData)
	}

	message := <-messages
	if !strings.Contains(message, "Subject: 批量订单通知\r\n") {
		t.Fatalf("SMTP batch message did not use mapped title:\n%s", message)
	}
	if strings.Contains(message, "Subject: order-created\r\n") {
		t.Fatalf("SMTP batch message used task alias as title:\n%s", message)
	}
}

func startSMTPRecorder(t *testing.T, messageCount int) (string, int, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen for SMTP recorder: %v", err)
	}
	t.Cleanup(func() { _ = listener.Close() })

	host, portText, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatalf("split SMTP recorder address: %v", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatalf("parse SMTP recorder port: %v", err)
	}
	messages := make(chan string, messageCount)

	go func() {
		defer close(messages)
		for i := 0; i < messageCount; i++ {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			recordSMTPMessage(conn, messages)
		}
	}()

	return host, port, messages
}

func recordSMTPMessage(conn net.Conn, messages chan<- string) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	_, _ = fmt.Fprint(conn, "220 localhost ESMTP ready\r\n")

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		command := strings.ToUpper(strings.TrimSpace(line))
		switch {
		case strings.HasPrefix(command, "EHLO"):
			_, _ = fmt.Fprint(conn, "250-localhost\r\n250 AUTH PLAIN\r\n")
		case strings.HasPrefix(command, "AUTH"):
			_, _ = fmt.Fprint(conn, "235 authenticated\r\n")
		case strings.HasPrefix(command, "MAIL FROM"):
			_, _ = fmt.Fprint(conn, "250 sender ok\r\n")
		case strings.HasPrefix(command, "RCPT TO"):
			_, _ = fmt.Fprint(conn, "250 recipient ok\r\n")
		case command == "DATA":
			_, _ = fmt.Fprint(conn, "354 end with dot\r\n")
			var message strings.Builder
			for {
				dataLine, readErr := reader.ReadString('\n')
				if readErr != nil {
					return
				}
				if dataLine == ".\r\n" {
					break
				}
				message.WriteString(dataLine)
			}
			messages <- message.String()
			_, _ = fmt.Fprint(conn, "250 queued\r\n")
		case command == "QUIT":
			_, _ = fmt.Fprint(conn, "221 bye\r\n")
			return
		default:
			_, _ = fmt.Fprint(conn, "250 ok\r\n")
		}
	}
}
