package service

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/dao"
	"cnb.cool/mliev/push/message-push/app/dto"
	"cnb.cool/mliev/push/message-push/app/model"
	registry "cnb.cool/mliev/push/message-push/modules/sender/domain"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

const (
	testSignatureRequiredProvider = "signature_service_required"
	testSignatureHistoryProvider  = "signature_service_history"
	testSignatureReadOnlyProvider = "signature_service_read_only"
)

func TestGlobalSignaturePaginationAndNestedCompatibility(t *testing.T) {
	registerSignatureServiceProviders(t)
	db := newSignatureServiceTestDB(t)
	requiredAccount := createSignatureTestAccount(t, db, "required", testSignatureRequiredProvider, constants.MessageTypeSMS)
	historyAccount := createSignatureTestAccount(t, db, "history", testSignatureHistoryProvider, constants.MessageTypeEmail)

	first := createSignatureRecord(t, db, requiredAccount.ID, "code-1", 1)
	second := createSignatureRecord(t, db, requiredAccount.ID, "code-2", 1)
	disabled := createSignatureRecord(t, db, requiredAccount.ID, "code-3", 1)
	if err := db.Model(&model.ProviderSignature{}).Where("id = ?", disabled.ID).Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}
	historical := createSignatureRecord(t, db, historyAccount.ID, "legacy-subject", 1)

	service := &AdminProviderSignatureService{
		signatureDAO: dao.NewProviderSignatureDAO(db),
		accountDAO:   dao.NewProviderAccountDAOWithDB(db),
	}

	pageOne, err := service.GetGlobalSignatureList(&dto.ProviderSignatureListRequest{
		ProviderAccountID: requiredAccount.ID,
		Page:              1,
		PageSize:          2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if pageOne.Total != 3 || pageOne.Page != 1 || pageOne.Size != 2 || len(pageOne.Items) != 2 {
		t.Fatalf("unexpected first page: %+v", pageOne)
	}
	if pageOne.Items[0].ID != disabled.ID || pageOne.Items[0].ProviderAccountName != requiredAccount.AccountName ||
		pageOne.Items[0].ProviderType != constants.MessageTypeSMS || !pageOne.Items[0].RequiresSignature {
		t.Fatalf("unexpected enriched item: %+v", pageOne.Items[0])
	}

	pageTwo, err := service.GetGlobalSignatureList(&dto.ProviderSignatureListRequest{
		ProviderAccountID: requiredAccount.ID,
		Page:              2,
		PageSize:          2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pageTwo.Items) != 1 || pageTwo.Items[0].ID != first.ID {
		t.Fatalf("unexpected second page: %+v", pageTwo)
	}

	active := int8(1)
	filtered, err := service.GetGlobalSignatureList(&dto.ProviderSignatureListRequest{
		ProviderAccountID: requiredAccount.ID,
		Status:            &active,
		Page:              1,
		PageSize:          20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if filtered.Total != 2 || len(filtered.Items) != 2 || filtered.Items[0].ID != second.ID {
		t.Fatalf("unexpected active filter: %+v", filtered)
	}

	nested, err := service.GetSignatureList(requiredAccount.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(nested) != 3 || !nested[0].RequiresSignature {
		t.Fatalf("nested list changed unexpectedly: %+v", nested)
	}
	raw, err := json.Marshal(nested)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || raw[0] != '[' {
		t.Fatalf("nested API must remain a bare array, got %s", raw)
	}

	historyPage, err := service.GetGlobalSignatureList(&dto.ProviderSignatureListRequest{
		ProviderAccountID: historyAccount.ID,
		Page:              1,
		PageSize:          20,
	})
	if err != nil {
		t.Fatal(err)
	}
	if historyPage.Total != 1 || historyPage.Items[0].ID != historical.ID ||
		historyPage.Items[0].RequiresSignature || !historyPage.Items[0].HistoricalOnly || !historyPage.Items[0].ReadOnly {
		t.Fatalf("unexpected historical policy: %+v", historyPage.Items)
	}
}

func TestNonSignatureProviderHistoryIsReadOnly(t *testing.T) {
	registerSignatureServiceProviders(t)
	db := newSignatureServiceTestDB(t)
	historyAccount := createSignatureTestAccount(t, db, "history", testSignatureHistoryProvider, constants.MessageTypeEmail)
	historical := createSignatureRecord(t, db, historyAccount.ID, "legacy", 1)
	service := &AdminProviderSignatureService{
		signatureDAO: dao.NewProviderSignatureDAO(db),
		accountDAO:   dao.NewProviderAccountDAOWithDB(db),
	}

	_, err := service.CreateSignature(historyAccount.ID, &dto.CreateProviderSignatureRequest{
		SignatureCode: "new",
		SignatureName: "new",
		Status:        1,
	})
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("create error = %v, want read-only policy", err)
	}
	err = service.UpdateSignature(historical.ID, &dto.UpdateProviderSignatureRequest{
		SignatureCode: "changed",
		SignatureName: "changed",
		Status:        1,
	})
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("update error = %v, want read-only policy", err)
	}
	err = service.DeleteSignature(historical.ID)
	if err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("delete error = %v, want read-only policy", err)
	}

	if _, err := service.GetSignatureByID(historical.ID); err != nil {
		t.Fatalf("historical GET must remain available: %v", err)
	}
	if _, err := service.GetSignatureList(historyAccount.ID, nil); err != nil {
		t.Fatalf("historical nested list must remain available: %v", err)
	}

	readOnlyAccount := createSignatureTestAccount(t, db, "read-only", testSignatureReadOnlyProvider, constants.MessageTypeWeChatWork)
	readOnlyRecord := createSignatureRecord(t, db, readOnlyAccount.ID, "legacy-wechat", 1)
	item, err := service.GetSignatureByID(readOnlyRecord.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !item.ReadOnly || item.HistoricalOnly || item.RequiresSignature {
		t.Fatalf("non-email signature policy = %+v, want read_only without historical_only", item)
	}
	if err := service.DeleteSignature(readOnlyRecord.ID); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("non-email delete error = %v, want read-only policy", err)
	}
}

func newSignatureServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "signature.db")), &gorm.Config{DisableForeignKeyConstraintWhenMigrating: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range []string{
		`CREATE TABLE provider_accounts (id INTEGER PRIMARY KEY AUTOINCREMENT, account_code TEXT NOT NULL UNIQUE, account_name TEXT NOT NULL, provider_code TEXT NOT NULL, provider_type TEXT NOT NULL, config TEXT, status INTEGER, remark TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
		`CREATE TABLE provider_signatures (id INTEGER PRIMARY KEY AUTOINCREMENT, provider_account_id INTEGER NOT NULL, signature_code TEXT NOT NULL, signature_name TEXT NOT NULL, status INTEGER, remark TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)`,
	} {
		if err := db.Exec(statement).Error; err != nil {
			t.Fatalf("create signature schema: %v", err)
		}
	}
	return db
}

func registerSignatureServiceProviders(t *testing.T) {
	t.Helper()
	providers := []*registry.ProviderMeta{
		{Code: testSignatureRequiredProvider, Name: "required", Type: constants.MessageTypeSMS, RequiresSignature: true},
		{Code: testSignatureHistoryProvider, Name: "history", Type: constants.MessageTypeEmail},
		{Code: testSignatureReadOnlyProvider, Name: "read-only", Type: constants.MessageTypeWeChatWork},
	}
	for _, provider := range providers {
		if err := registry.Register(provider); err != nil && !strings.Contains(err.Error(), "already registered") {
			t.Fatalf("register provider %s: %v", provider.Code, err)
		}
	}
}

func createSignatureTestAccount(t *testing.T, db *gorm.DB, code, providerCode, providerType string) *model.ProviderAccount {
	t.Helper()
	account := &model.ProviderAccount{
		AccountCode:  code,
		AccountName:  code,
		ProviderCode: providerCode,
		ProviderType: providerType,
		Config:       `{}`,
		Status:       1,
	}
	if err := db.Create(account).Error; err != nil {
		t.Fatal(err)
	}
	return account
}

func createSignatureRecord(t *testing.T, db *gorm.DB, accountID uint, code string, status int8) *model.ProviderSignature {
	t.Helper()
	signature := &model.ProviderSignature{
		ProviderAccountID: accountID,
		SignatureCode:     code,
		SignatureName:     code,
		Status:            status,
	}
	if err := db.Create(signature).Error; err != nil {
		t.Fatal(err)
	}
	return signature
}
