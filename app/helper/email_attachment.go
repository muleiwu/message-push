package helper

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"mime"
	"net/http"
	"strings"
	"unicode"
	"unicode/utf8"

	webHelper "cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
)

const (
	defaultEmailAttachmentMaxCount       = 5
	defaultEmailAttachmentMaxFileSizeMiB = 5
	defaultEmailAttachmentMaxTotalMiB    = 10
	attachmentRequestMetadataAllowance   = int64(1 << 20)
	bytesPerMiB                          = int64(1 << 20)
)

// EmailAttachmentLimits 是解码后附件内容的容量限制。
type EmailAttachmentLimits struct {
	MaxCount      int
	MaxFileBytes  int64
	MaxTotalBytes int64
}

// GetEmailAttachmentLimits 从运行时配置读取附件限制，非法配置回退到安全默认值。
func GetEmailAttachmentLimits() EmailAttachmentLimits {
	config := webHelper.GetConfig()
	maxCount := config.GetInt("email.attachments.max_count", defaultEmailAttachmentMaxCount)
	maxFileMiB := config.GetInt("email.attachments.max_file_size_mb", defaultEmailAttachmentMaxFileSizeMiB)
	maxTotalMiB := config.GetInt("email.attachments.max_total_size_mb", defaultEmailAttachmentMaxTotalMiB)

	if maxCount <= 0 {
		maxCount = defaultEmailAttachmentMaxCount
	}
	if maxFileMiB <= 0 {
		maxFileMiB = defaultEmailAttachmentMaxFileSizeMiB
	}
	if maxTotalMiB <= 0 {
		maxTotalMiB = defaultEmailAttachmentMaxTotalMiB
	}
	if maxFileMiB > maxTotalMiB {
		maxFileMiB = maxTotalMiB
	}

	return EmailAttachmentLimits{
		MaxCount:      maxCount,
		MaxFileBytes:  int64(maxFileMiB) * bytesPerMiB,
		MaxTotalBytes: int64(maxTotalMiB) * bytesPerMiB,
	}
}

// MaxJSONBodyBytes 返回足以容纳 Base64 附件和常规 JSON 字段的请求体上限。
func (l EmailAttachmentLimits) MaxJSONBodyBytes() int64 {
	encoded := ((l.MaxTotalBytes + 2) / 3) * 4
	return encoded + attachmentRequestMetadataAllowance
}

// DecodeEmailAttachments 校验并解码请求附件。返回值尚未设置附件组 ID。
func DecodeEmailAttachments(requests []dto.EmailAttachmentRequest, limits EmailAttachmentLimits) ([]*model.EmailAttachment, error) {
	if len(requests) == 0 {
		return nil, nil
	}
	if len(requests) > limits.MaxCount {
		return nil, fmt.Errorf("too many email attachments: got %d, maximum is %d", len(requests), limits.MaxCount)
	}

	attachments := make([]*model.EmailAttachment, 0, len(requests))
	var totalBytes int64
	for index, request := range requests {
		filename, err := validateAttachmentFilename(request.Filename)
		if err != nil {
			return nil, fmt.Errorf("invalid email attachment %d filename: %w", index+1, err)
		}

		content, err := base64.StdEncoding.DecodeString(request.ContentBase64)
		if err != nil {
			return nil, fmt.Errorf("invalid email attachment %d base64 content", index+1)
		}
		if len(content) == 0 {
			return nil, fmt.Errorf("email attachment %d must not be empty", index+1)
		}
		size := int64(len(content))
		if size > limits.MaxFileBytes {
			return nil, fmt.Errorf("email attachment %d exceeds maximum file size of %d bytes", index+1, limits.MaxFileBytes)
		}
		totalBytes += size
		if totalBytes > limits.MaxTotalBytes {
			return nil, fmt.Errorf("email attachments exceed maximum total size of %d bytes", limits.MaxTotalBytes)
		}

		contentType, err := normalizeAttachmentContentType(request.ContentType, content)
		if err != nil {
			return nil, fmt.Errorf("invalid email attachment %d content_type: %w", index+1, err)
		}
		digest := sha256.Sum256(content)
		attachments = append(attachments, &model.EmailAttachment{
			Position:    index,
			Filename:    filename,
			ContentType: contentType,
			SizeBytes:   size,
			SHA256:      hex.EncodeToString(digest[:]),
			Content:     content,
		})
	}
	return attachments, nil
}

func validateAttachmentFilename(value string) (string, error) {
	filename := strings.TrimSpace(value)
	if filename == "" {
		return "", fmt.Errorf("must not be empty")
	}
	if !utf8.ValidString(filename) {
		return "", fmt.Errorf("must be valid UTF-8")
	}
	if len([]byte(filename)) > 255 {
		return "", fmt.Errorf("must not exceed 255 bytes")
	}
	if filename == "." || filename == ".." || strings.ContainsAny(filename, "/\\") {
		return "", fmt.Errorf("must be a plain filename without path separators")
	}
	for _, character := range filename {
		if unicode.IsControl(character) {
			return "", fmt.Errorf("must not contain control characters")
		}
	}
	return filename, nil
}

func normalizeAttachmentContentType(value string, content []byte) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return http.DetectContentType(content), nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("must not contain CR or LF characters")
	}
	mediaType, params, err := mime.ParseMediaType(value)
	if err != nil || mediaType == "" {
		return "", fmt.Errorf("must be a valid MIME media type")
	}
	return mime.FormatMediaType(mediaType, params), nil
}
