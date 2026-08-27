package infrastructure

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"net/textproto"
	"strings"
	"unicode"

	"cnb.cool/mliev/push/message-push/app/model"
)

func buildSMTPMessage(from, to, subject, bodyContentType, body string, attachments []*model.EmailAttachment) ([]byte, error) {
	fromHeader, err := normalizeAddressHeader("from", from)
	if err != nil {
		return nil, err
	}
	toHeader, err := normalizeAddressHeader("to", to)
	if err != nil {
		return nil, err
	}
	encodedSubject, err := encodeSubjectHeader(subject)
	if err != nil {
		return nil, err
	}
	bodyContentType, err = normalizeMIMEContentType(bodyContentType)
	if err != nil {
		return nil, fmt.Errorf("invalid email body content type: %w", err)
	}

	var message bytes.Buffer
	writeHeaderLine(&message, "From", fromHeader)
	writeHeaderLine(&message, "To", toHeader)
	writeHeaderLine(&message, "Subject", encodedSubject)
	writeHeaderLine(&message, "MIME-Version", "1.0")

	if len(attachments) == 0 {
		writeHeaderLine(&message, "Content-Type", bodyContentType)
		message.WriteString("\r\n")
		message.WriteString(body)
		return message.Bytes(), nil
	}

	multipartWriter := multipart.NewWriter(&message)
	writeHeaderLine(&message, "Content-Type", mime.FormatMediaType("multipart/mixed", map[string]string{"boundary": multipartWriter.Boundary()}))
	message.WriteString("\r\n")

	bodyHeader := make(textproto.MIMEHeader)
	bodyHeader.Set("Content-Type", bodyContentType)
	bodyHeader.Set("Content-Transfer-Encoding", "quoted-printable")
	bodyPart, err := multipartWriter.CreatePart(bodyHeader)
	if err != nil {
		return nil, fmt.Errorf("create email body part: %w", err)
	}
	quotedBody := quotedprintable.NewWriter(bodyPart)
	if _, err := quotedBody.Write([]byte(body)); err != nil {
		return nil, fmt.Errorf("write email body part: %w", err)
	}
	if err := quotedBody.Close(); err != nil {
		return nil, fmt.Errorf("close email body part: %w", err)
	}

	for _, attachment := range attachments {
		if err := writeAttachmentPart(multipartWriter, attachment); err != nil {
			return nil, err
		}
	}
	if err := multipartWriter.Close(); err != nil {
		return nil, fmt.Errorf("close multipart email: %w", err)
	}
	return message.Bytes(), nil
}

func writeAttachmentPart(writer *multipart.Writer, attachment *model.EmailAttachment) error {
	if attachment == nil || len(attachment.Content) == 0 {
		return fmt.Errorf("email attachment content must not be empty")
	}
	filename, err := normalizeMIMEFilename(attachment.Filename)
	if err != nil {
		return err
	}
	mediaType, params, err := mime.ParseMediaType(attachment.ContentType)
	if err != nil || mediaType == "" {
		return fmt.Errorf("invalid email attachment %q content type", filename)
	}
	params["name"] = filename

	header := make(textproto.MIMEHeader)
	header.Set("Content-Type", mime.FormatMediaType(mediaType, params))
	header.Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": filename}))
	header.Set("Content-Transfer-Encoding", "base64")
	part, err := writer.CreatePart(header)
	if err != nil {
		return fmt.Errorf("create email attachment %q: %w", filename, err)
	}

	encoded := base64.StdEncoding.EncodeToString(attachment.Content)
	for len(encoded) > 76 {
		if _, err := fmt.Fprintf(part, "%s\r\n", encoded[:76]); err != nil {
			return fmt.Errorf("write email attachment %q: %w", filename, err)
		}
		encoded = encoded[76:]
	}
	if _, err := fmt.Fprintf(part, "%s\r\n", encoded); err != nil {
		return fmt.Errorf("write email attachment %q: %w", filename, err)
	}
	return nil
}

func normalizeAddressHeader(name, value string) (string, error) {
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("email %s address must not contain CR or LF characters", name)
	}
	address, err := mail.ParseAddress(strings.TrimSpace(value))
	if err != nil {
		return "", fmt.Errorf("invalid email %s address: %w", name, err)
	}
	return address.String(), nil
}

func encodeSubjectHeader(subject string) (string, error) {
	if strings.ContainsAny(subject, "\r\n") {
		return "", fmt.Errorf("email subject must not contain CR or LF characters")
	}
	for _, character := range subject {
		if character > unicode.MaxASCII {
			return mime.QEncoding.Encode("UTF-8", subject), nil
		}
	}
	return subject, nil
}

func normalizeMIMEContentType(value string) (string, error) {
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("content type must not contain CR or LF characters")
	}
	mediaType, params, err := mime.ParseMediaType(value)
	if err != nil || mediaType == "" {
		return "", fmt.Errorf("content type must be a valid MIME media type")
	}
	return mime.FormatMediaType(mediaType, params), nil
}

func normalizeMIMEFilename(value string) (string, error) {
	filename := strings.TrimSpace(value)
	if filename == "" || filename == "." || filename == ".." || strings.ContainsAny(filename, "/\\") {
		return "", fmt.Errorf("invalid email attachment filename")
	}
	for _, character := range filename {
		if unicode.IsControl(character) {
			return "", fmt.Errorf("invalid email attachment filename")
		}
	}
	return filename, nil
}

func writeHeaderLine(buffer *bytes.Buffer, name, value string) {
	fmt.Fprintf(buffer, "%s: %s\r\n", name, value)
}

func attachmentAuditData(attachments []*model.EmailAttachment) []map[string]any {
	if len(attachments) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(attachments))
	for _, attachment := range attachments {
		if attachment == nil {
			continue
		}
		result = append(result, map[string]any{
			"filename":     attachment.Filename,
			"content_type": attachment.ContentType,
			"size_bytes":   attachment.SizeBytes,
			"sha256":       attachment.SHA256,
		})
	}
	return result
}
