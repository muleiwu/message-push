package readiness

import (
	"reflect"
	"testing"

	"cnb.cool/mliev/push/message-push/app/constants"
	"cnb.cool/mliev/push/message-push/app/model"
	"gorm.io/gorm"
)

func TestOneBotOptionalAliasesRequireEveryEligibleAccount(t *testing.T) {
	db := newReadinessTestDB(t)
	f := createReadyFixture(t, db, constants.MessageTypeQQ, constants.ProviderOneBot)
	e := NewChannelEvaluator(db)
	assert := func(wantAccounts, wantBindings int, aliases []string) {
		t.Helper()
		state, err := e.EvaluateChannel(f.channel.ID)
		if err != nil {
			t.Fatal(err)
		}
		if state.State != constants.ChannelReadinessReady || state.RequiredSignatureAccountCount != 0 || state.OptionalSignatureAccountCount != wantAccounts || state.ValidBindingCount != wantBindings || !reflect.DeepEqual(state.OptionalSignatureAliases, aliases) {
			t.Fatalf("readiness=%+v want aliases=%v", state, aliases)
		}
		if err := e.ValidateForSend(f.channel.ID, " "); err != nil {
			t.Fatalf("unsigned send blocked: %v", err)
		}
		for _, alias := range aliases {
			if err := e.ValidateForSend(f.channel.ID, " "+alias+" "); err != nil {
				t.Fatalf("valid alias rejected: %v", err)
			}
		}
		if err := e.ValidateForSend(f.channel.ID, "unknown"); !validationHasCode(err, constants.ReadinessBlockerSignatureAliasNotCommon) {
			t.Fatalf("unknown alias error=%v", err)
		}
	}
	assert(1, 1, []string{})
	createSignatureAlias(t, db, f.channel.ID, f.account.ID, "notice")
	createSignatureAlias(t, db, f.channel.ID, f.account.ID, "primary-only")
	assert(1, 1, []string{"notice", "primary-only"})
	account, _, binding := createProviderPath(t, db, f.channel.ID, constants.MessageTypeQQ, constants.ProviderOneBot)
	assert(2, 2, []string{})
	if err := e.ValidateForSend(f.channel.ID, "notice"); err == nil {
		t.Fatal("accepted an alias missing on the second account")
	}
	createSignatureAlias(t, db, f.channel.ID, account.ID, "notice")
	createSignatureAlias(t, db, f.channel.ID, account.ID, "backup-only")
	assert(2, 2, []string{"notice"})
	// Multiple bindings for one account do not add another signature requirement.
	createBinding(t, db, f.channel.ID, f.providerTemplate.ID, f.account.ID, 1, 1, f.binding.ParamMapping)
	assert(2, 3, []string{"notice"})
	if err := db.Model(binding).Update("status", 0).Error; err != nil {
		t.Fatal(err)
	}
	assert(1, 2, []string{"notice", "primary-only"})
}

func TestOneBotInvalidMappingNeverBlocksUnsignedDelivery(t *testing.T) {
	for _, tt := range []struct {
		name   string
		change func(*gorm.DB, *model.ChannelSignatureMapping) error
	}{
		{"disabled mapping", func(db *gorm.DB, m *model.ChannelSignatureMapping) error {
			return db.Model(m).Update("status", 0).Error
		}},
		{"deleted mapping", func(db *gorm.DB, m *model.ChannelSignatureMapping) error { return db.Delete(m).Error }},
		{"disabled signature", func(db *gorm.DB, m *model.ChannelSignatureMapping) error {
			return db.Model(&model.ProviderSignature{}).Where("id = ?", m.ProviderSignatureID).Update("status", 0).Error
		}},
		{"deleted signature", func(db *gorm.DB, m *model.ChannelSignatureMapping) error {
			return db.Delete(&model.ProviderSignature{}, m.ProviderSignatureID).Error
		}},
		{"empty signature", func(db *gorm.DB, m *model.ChannelSignatureMapping) error {
			return db.Model(&model.ProviderSignature{}).Where("id = ?", m.ProviderSignatureID).Update("signature_code", " \n").Error
		}},
		{"wrong owner", func(db *gorm.DB, m *model.ChannelSignatureMapping) error {
			return db.Model(&model.ProviderSignature{}).Where("id = ?", m.ProviderSignatureID).Update("provider_account_id", 999).Error
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			db := newReadinessTestDB(t)
			f := createReadyFixture(t, db, constants.MessageTypeQQ, constants.ProviderOneBot)
			mapping := createSignatureAlias(t, db, f.channel.ID, f.account.ID, "notice")
			if err := tt.change(db, mapping); err != nil {
				t.Fatal(err)
			}
			e := NewChannelEvaluator(db)
			if err := e.ValidateForSend(f.channel.ID, ""); err != nil {
				t.Fatalf("unsigned request blocked: %v", err)
			}
			if err := e.ValidateForSend(f.channel.ID, "notice"); !validationHasCode(err, constants.ReadinessBlockerSignatureAliasNotCommon) {
				t.Fatalf("invalid signed request: %v", err)
			}
		})
	}
}
