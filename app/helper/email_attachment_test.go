package helper

import (
	"encoding/base64"
	"strings"
	"testing"

	"cnb.cool/mliev/push/message-push/app/dto"
)

func TestDecodeEmailAttachmentsValidatesAndHashesDecodedContent(t *testing.T) {
	attachments, err := DecodeEmailAttachments([]dto.EmailAttachmentRequest{{
		Filename:      "报告.txt",
		ContentBase64: base64.StdEncoding.EncodeToString([]byte("hello")),
	}}, EmailAttachmentLimits{MaxCount: 1, MaxFileBytes: 5, MaxTotalBytes: 5})
	if err != nil {
		t.Fatalf("DecodeEmailAttachments() error = %v", err)
	}
	if len(attachments) != 1 || attachments[0].ContentType != "text/plain; charset=utf-8" || attachments[0].SizeBytes != 5 {
		t.Fatalf("decoded attachment = %+v", attachments)
	}
	if attachments[0].SHA256 != "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824" {
		t.Fatalf("sha256 = %q", attachments[0].SHA256)
	}
}

func TestDecodeEmailAttachmentsRejectsUnsafeAndOversizedInput(t *testing.T) {
	limits := EmailAttachmentLimits{MaxCount: 1, MaxFileBytes: 2, MaxTotalBytes: 2}
	tests := []struct {
		name    string
		request dto.EmailAttachmentRequest
		want    string
	}{
		{name: "path filename", request: dto.EmailAttachmentRequest{Filename: "../secret.txt", ContentBase64: "eA=="}, want: "path separators"},
		{name: "header filename", request: dto.EmailAttachmentRequest{Filename: "safe.txt\r\nBcc: bad@example.com", ContentBase64: "eA=="}, want: "control characters"},
		{name: "invalid base64", request: dto.EmailAttachmentRequest{Filename: "safe.txt", ContentBase64: "%%%"}, want: "base64"},
		{name: "empty", request: dto.EmailAttachmentRequest{Filename: "safe.txt", ContentBase64: ""}, want: "must not be empty"},
		{name: "oversized", request: dto.EmailAttachmentRequest{Filename: "safe.txt", ContentBase64: base64.StdEncoding.EncodeToString([]byte("abc"))}, want: "maximum file size"},
		{name: "invalid mime", request: dto.EmailAttachmentRequest{Filename: "safe.txt", ContentType: "text/plain\r\nX-Bad: yes", ContentBase64: "eA=="}, want: "content_type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := DecodeEmailAttachments([]dto.EmailAttachmentRequest{tt.request}, limits)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want containing %q", err, tt.want)
			}
		})
	}
}

func TestEmailAttachmentLimitsDeriveEncodedBodyAllowance(t *testing.T) {
	limits := EmailAttachmentLimits{MaxTotalBytes: 10}
	if got := limits.MaxJSONBodyBytes(); got != 16+attachmentRequestMetadataAllowance {
		t.Fatalf("MaxJSONBodyBytes() = %d", got)
	}
}
