package dto

import (
	"strings"
	"testing"

	"github.com/go-playground/validator/v10"
)

func TestProviderSignatureFieldLengthContract(t *testing.T) {
	validate := validator.New()
	validate.SetTagName("binding")

	request := CreateProviderSignatureRequest{
		SignatureCode: strings.Repeat("题", 200),
		SignatureName: strings.Repeat("n", 100),
	}
	if err := validate.Struct(request); err != nil {
		t.Fatalf("maximum-length provider signature was rejected: %v", err)
	}

	request.SignatureCode = strings.Repeat("题", 201)
	if err := validate.Struct(request); err == nil {
		t.Fatal("expected a 201-character mapped value to be rejected")
	}

	request.SignatureCode = "邮件标题"
	request.SignatureName = strings.Repeat("n", 101)
	if err := validate.Struct(request); err == nil {
		t.Fatal("expected a 101-character display name to be rejected")
	}
}
