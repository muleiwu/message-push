package service

import (
	"errors"
	"fmt"

	"cnb.cool/mliev/push/message-push/app/model"
	"cnb.cool/mliev/push/message-push/app/readiness"
	registry "cnb.cool/mliev/push/message-push/modules/sender/domain"
)

var ErrBindingContentConflict = errors.New("模板正文已变化，请重新加载原文并确认参数映射")

func confirmBindingContent(binding *model.ChannelTemplateBinding, version *uint64) error {
	if !registry.IsSMSTemplate(binding.ProviderTemplate) {
		return nil
	}
	if version == nil || *version == 0 || *version != binding.ProviderTemplate.ContentVersion {
		return ErrBindingContentConflict
	}
	binding.MappedContentVersion = *version
	return nil
}

func validateBindingCandidate(binding *model.ChannelTemplateBinding) error {
	if binding.Channel == nil || binding.Channel.MessageTemplate == nil {
		return fmt.Errorf("channel message template not found")
	}
	variables, err := binding.Channel.MessageTemplate.GetVariables()
	if err != nil {
		return err
	}
	var issues []string
	if binding.Status == 1 && binding.IsActive == 1 {
		issues = readiness.ValidateBinding(binding.Channel.Type, variables, binding)
	} else {
		issues = readiness.ValidateBindingParamMapping(variables, binding)
	}
	if len(issues) > 0 {
		return newChannelBindingValidationError(binding, issues)
	}
	return nil
}
