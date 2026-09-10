package infrastructure

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"
	"testing"

	"cnb.cool/mliev/push/message-push/app/model"
)

func TestBuildSMTPMessageCreatesMultipartMixedWithUnicodeAttachment(t *testing.T) {
	attachmentContent := []byte{0x00, 0x01, 0x02, 0xfe, 0xff}
	message, err := buildSMTPMessage(
		"sender@example.com",
		"recipient@example.com",
		"附件测试",
		"text/html; charset=UTF-8",
		"<strong>正文</strong>",
		[]*model.EmailAttachment{{
			Filename:    "报告.bin",
			ContentType: "application/octet-stream",
			Content:     attachmentContent,
		}},
	)
	if err != nil {
		t.Fatalf("buildSMTPMessage() error = %v", err)
	}

	parsed, err := mail.ReadMessage(bytes.NewReader(message))
	if err != nil {
		t.Fatalf("parse message: %v", err)
	}
	subject, err := new(mime.WordDecoder).DecodeHeader(parsed.Header.Get("Subject"))
	if err != nil || subject != "附件测试" {
		t.Fatalf("decoded subject = %q, %v", subject, err)
	}
	mediaType, params, err := mime.ParseMediaType(parsed.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/mixed" {
		t.Fatalf("message content type = %q, %v", mediaType, err)
	}
	reader := multipart.NewReader(parsed.Body, params["boundary"])

	bodyPart, err := reader.NextPart()
	if err != nil {
		t.Fatalf("read body part: %v", err)
	}
	body, err := io.ReadAll(quotedprintable.NewReader(bodyPart))
	if err != nil || string(body) != "<strong>正文</strong>" {
		t.Fatalf("decoded body = %q, %v", body, err)
	}

	attachmentPart, err := reader.NextPart()
	if err != nil {
		t.Fatalf("read attachment part: %v", err)
	}
	_, dispositionParams, err := mime.ParseMediaType(attachmentPart.Header.Get("Content-Disposition"))
	if err != nil || dispositionParams["filename"] != "报告.bin" {
		t.Fatalf("attachment filename = %q, %v", dispositionParams["filename"], err)
	}
	decoded, err := io.ReadAll(base64.NewDecoder(base64.StdEncoding, attachmentPart))
	if err != nil || !bytes.Equal(decoded, attachmentContent) {
		t.Fatalf("decoded attachment = %v, %v", decoded, err)
	}
	if _, err := reader.NextPart(); err != io.EOF {
		t.Fatalf("final multipart read error = %v, want EOF", err)
	}
}

func TestBuildSMTPMessageRejectsHeaderInjectionAndKeepsAuditContentFree(t *testing.T) {
	if _, err := buildSMTPMessage("sender@example.com\r\nBcc: bad@example.com", "recipient@example.com", "subject", "text/plain", "body", nil); err == nil {
		t.Fatal("expected from header injection to be rejected")
	}
	if _, err := buildSMTPMessage("sender@example.com", "recipient@example.com", "subject", "text/plain", "body", []*model.EmailAttachment{{
		Filename: "bad\r\nX-Test: yes",
		Content:  []byte("secret-content"),
	}}); err == nil {
		t.Fatal("expected filename header injection to be rejected")
	}
	audit, err := json.Marshal(attachmentAuditData([]*model.EmailAttachment{{Filename: "safe.txt", Content: []byte("secret-content"), SizeBytes: 14}}))
	if err != nil {
		t.Fatalf("marshal attachment audit data: %v", err)
	}
	if strings.Contains(string(audit), "secret-content") {
		t.Fatal("attachment audit data leaked content")
	}
}
