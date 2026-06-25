package infrastructure

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"cnb.cool/mliev/open/go-web/pkg/helper"
	"cnb.cool/mliev/push/message-push/app/constants"
	domain "cnb.cool/mliev/push/message-push/modules/sender/domain"
)

func init() {
	// 注册钉钉服务商
	domain.Register(&domain.ProviderMeta{
		Code:        constants.ProviderDingTalk,
		Name:        "钉钉",
		Type:        constants.MessageTypeDingTalk,
		Description: "钉钉工作通知消息服务，支持文本和Markdown消息",
		ConfigFields: []domain.ConfigField{
			{
				Key:         "app_key",
				Label:       "应用AppKey",
				Description: "钉钉应用的AppKey",
				Type:        domain.FieldTypeText,
				Required:    true,
				Example:     "dingxxxxxxxxxxxxxx",
				Placeholder: "请输入AppKey",
				HelpLink:    "https://open.dingtalk.com/document/orgapp/obtain-the-access_token-of-an-internal-app",
			},
			{
				Key:         "app_secret",
				Label:       "应用AppSecret",
				Description: "钉钉应用的AppSecret",
				Type:        domain.FieldTypePassword,
				Required:    true,
				Example:     "xxxxxxxxxxxxxxxxxxxxxx",
				Placeholder: "请输入AppSecret",
				HelpLink:    "https://open.dingtalk.com/document/orgapp/obtain-the-access_token-of-an-internal-app",
			},
			{
				Key:         "agent_id",
				Label:       "应用AgentId",
				Description: "钉钉应用的AgentId",
				Type:        domain.FieldTypeText,
				Required:    true,
				Example:     "123456789",
				Placeholder: "请输入AgentId",
			},
			{
				Key:         "callback_token",
				Label:       "回调Token",
				Description: "事件订阅的Token，用于签名验证",
				Type:        domain.FieldTypeText,
				Required:    false,
				Placeholder: "请输入回调Token",
			},
			{
				Key:         "callback_aes_key",
				Label:       "回调AESKey",
				Description: "事件订阅的EncodingAESKey（43位字符）",
				Type:        domain.FieldTypePassword,
				Required:    false,
				Placeholder: "请输入43位AESKey",
			},
		},
		// 能力声明
		SupportsSend:      true,
		SupportsBatchSend: true,
		SupportsCallback:  true,
		// 扩展信息
		Website:    "https://www.dingtalk.com/",
		Icon:       "https://www.dingtalk.com/favicon.ico",
		DocsUrl:    "https://open.dingtalk.com/document/orgapp/asynchronous-sending-of-enterprise-session-messages",
		ConsoleUrl: "https://open-dev.dingtalk.com/",
		PricingUrl: "",
		SortOrder:  40,
		Tags:       []string{"企业", "即时通讯"},
		Regions:    []string{"中国大陆"},
		Deprecated: false,
	})
}

type DingTalkSender struct {
}

func NewDingTalkSender() *DingTalkSender {
	return &DingTalkSender{}
}

func (s *DingTalkSender) GetProviderCode() string {
	return constants.ProviderDingTalk
}

func (s *DingTalkSender) Send(ctx context.Context, req *domain.SendRequest) (*domain.SendResponse, error) {
	config, err := req.ProviderAccount.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	appKey, _ := config["app_key"].(string)
	appSecret, _ := config["app_secret"].(string)
	agentID, _ := config["agent_id"].(string)

	if appKey == "" || appSecret == "" || agentID == "" {
		return nil, fmt.Errorf("missing dingtalk config: app_key, app_secret or agent_id")
	}

	// 1. 获取 Access Token
	token, err := s.getAccessToken(ctx, appKey, appSecret)
	if err != nil {
		return nil, err
	}

	// 2. 构造消息
	// 消息格式由供应商模板的 ContentType 决定，支持 text 和 markdown
	msgType := "text"
	if isMarkdownContent(req.ChannelTemplateBinding) {
		msgType = "markdown"
	}

	// 接收者 user_id_list
	receiver := req.Task.Receiver // userId1,userId2...
	content := req.RenderedContent

	// 获取标题（用于 markdown），从签名字段获取，默认为"消息"
	title := req.Task.Signature
	if title == "" {
		title = "消息"
	}

	// 构造消息体
	var msgContent map[string]interface{}
	if msgType == "markdown" {
		msgContent = map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"title": title,
				"text":  content,
			},
		}
	} else {
		msgContent = map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": content,
			},
		}
	}

	payload := map[string]interface{}{
		"agent_id":    agentID,
		"userid_list": receiver,
		"msg":         msgContent,
	}

	body, _ := json.Marshal(payload)

	// 3. 发送请求（使用 context）
	apiURL := fmt.Sprintf("https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2?access_token=%s", token)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var respData struct {
		ErrCode   int64  `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		TaskID    int64  `json:"task_id"`
		RequestID string `json:"request_id"`
	}

	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	if respData.ErrCode != 0 {
		return &domain.SendResponse{
			Success:      false,
			ErrorCode:    fmt.Sprintf("%d", respData.ErrCode),
			ErrorMessage: respData.ErrMsg,
			TaskID:       req.Task.TaskID,
			RequestData:  string(body),
			ResponseData: string(respBody),
		}, nil
	}

	return &domain.SendResponse{
		Success:      true,
		ProviderID:   fmt.Sprintf("%d", respData.TaskID),
		TaskID:       req.Task.TaskID,
		Status:       constants.TaskStatusSuccess, // 钉钉消息发送成功即完成
		RequestData:  string(body),
		ResponseData: string(respBody),
	}, nil
}

func (s *DingTalkSender) getAccessToken(ctx context.Context, appKey, appSecret string) (string, error) {
	redisClient := helper.GetRedis()
	key := fmt.Sprintf("dingtalk:token:%s", appKey)

	// 尝试从Redis获取
	token, err := redisClient.Get(ctx, key).Result()
	if err == nil && token != "" {
		return token, nil
	}

	// 从API获取（使用 context）
	apiURL := fmt.Sprintf("https://oapi.dingtalk.com/gettoken?appkey=%s&appsecret=%s", appKey, appSecret)
	httpReq, err := http.NewRequestWithContext(ctx, "GET", apiURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var data struct {
		ErrCode     int    `json:"errcode"`
		ErrMsg      string `json:"errmsg"`
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	if data.ErrCode != 0 {
		return "", fmt.Errorf("failed to get access token: %s", data.ErrMsg)
	}

	// 缓存到Redis
	redisClient.Set(ctx, key, data.AccessToken, time.Duration(data.ExpiresIn-200)*time.Second)

	return data.AccessToken, nil
}

// ==================== BatchSender 接口实现 ====================

// SupportsBatchSend 是否支持批量发送
func (s *DingTalkSender) SupportsBatchSend() bool {
	return true
}

// BatchSend 批量发送钉钉消息（钉钉支持 userid_list 用逗号分隔多个用户）
func (s *DingTalkSender) BatchSend(ctx context.Context, req *domain.BatchSendRequest) (*domain.BatchSendResponse, error) {
	if len(req.Tasks) == 0 {
		return &domain.BatchSendResponse{Results: []*domain.SendResponse{}}, nil
	}

	config, err := req.ProviderAccount.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("invalid provider config: %w", err)
	}

	appKey, _ := config["app_key"].(string)
	appSecret, _ := config["app_secret"].(string)
	agentID, _ := config["agent_id"].(string)

	if appKey == "" || appSecret == "" || agentID == "" {
		return nil, fmt.Errorf("missing dingtalk config: app_key, app_secret or agent_id")
	}

	// 获取 Access Token
	token, err := s.getAccessToken(ctx, appKey, appSecret)
	if err != nil {
		return nil, err
	}

	// 收集所有用户ID（用逗号分隔）
	var userIDs []string
	for _, task := range req.Tasks {
		if task.Receiver != "" {
			userIDs = append(userIDs, task.Receiver)
		}
	}
	userIDList := strings.Join(userIDs, ",")

	// 构造消息（批量发送时使用第一个任务的内容）
	firstTask := req.Tasks[0]

	// 消息格式由供应商模板的 ContentType 决定
	msgType := "text"
	if isMarkdownContent(req.ChannelTemplateBinding) {
		msgType = "markdown"
	}

	content := req.RenderedContent

	// 获取标题（用于 markdown），从签名字段获取，默认为"消息"
	title := firstTask.Signature
	if title == "" {
		title = "消息"
	}

	// 构造消息体
	var msgContent map[string]interface{}
	if msgType == "markdown" {
		msgContent = map[string]interface{}{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"title": title,
				"text":  content,
			},
		}
	} else {
		msgContent = map[string]interface{}{
			"msgtype": "text",
			"text": map[string]string{
				"content": content,
			},
		}
	}

	payload := map[string]interface{}{
		"agent_id":    agentID,
		"userid_list": userIDList,
		"msg":         msgContent,
	}

	body, _ := json.Marshal(payload)

	// 发送请求（使用 context）
	apiURL := fmt.Sprintf("https://oapi.dingtalk.com/topapi/message/corpconversation/asyncsend_v2?access_token=%s", token)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewBuffer(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var respData struct {
		ErrCode   int64  `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		TaskID    int64  `json:"task_id"`
		RequestID string `json:"request_id"`
	}

	if err := json.Unmarshal(respBody, &respData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %v", err)
	}

	// 构造结果
	results := make([]*domain.SendResponse, len(req.Tasks))
	providerID := fmt.Sprintf("%d", respData.TaskID)

	for i, task := range req.Tasks {
		if respData.ErrCode != 0 {
			results[i] = &domain.SendResponse{
				Success:      false,
				ErrorCode:    fmt.Sprintf("%d", respData.ErrCode),
				ErrorMessage: respData.ErrMsg,
				TaskID:       task.TaskID,
				RequestData:  string(body),
				ResponseData: string(respBody),
			}
		} else {
			results[i] = &domain.SendResponse{
				Success:      true,
				ProviderID:   providerID,
				TaskID:       task.TaskID,
				Status:       constants.TaskStatusSuccess, // 钉钉消息发送成功即完成
				RequestData:  string(body),
				ResponseData: string(respBody),
			}
		}
	}

	return &domain.BatchSendResponse{Results: results}, nil
}

// ==================== CallbackHandler 接口实现 ====================

// SupportsCallback 是否支持回调
func (s *DingTalkSender) SupportsCallback() bool {
	return true
}

// HandleCallback 处理钉钉回调
// 钉钉工作通知消息发送结果回调
func (s *DingTalkSender) HandleCallback(ctx context.Context, req *domain.CallbackRequest) (domain.CallbackResponse, []*domain.CallbackResult, error) {
	// 默认响应（钉钉期望返回加密后的 "success"）
	resp := domain.CallbackResponse{
		StatusCode: 200,
		Body:       "success",
	}

	// 解析请求体获取加密数据
	var encryptedReq struct {
		Encrypt string `json:"encrypt"`
	}
	if err := json.Unmarshal(req.RawBody, &encryptedReq); err != nil {
		return resp, nil, fmt.Errorf("invalid callback request: %w", err)
	}

	// 获取回调签名验证参数
	signature := req.QueryParams["signature"]
	timestamp := req.QueryParams["timestamp"]
	nonce := req.QueryParams["nonce"]

	// 从 Headers 获取配置信息（需要上层服务注入）
	token := req.Headers["X-Callback-Token"]
	aesKey := req.Headers["X-Callback-AesKey"]
	corpID := req.Headers["X-Callback-CorpId"]

	// 如果没有加密配置，尝试直接解析（兼容旧版本）
	if aesKey == "" || encryptedReq.Encrypt == "" {
		return s.handlePlainCallback(req.RawBody)
	}

	// 验证签名
	if !s.verifyCallbackSignature(token, timestamp, nonce, encryptedReq.Encrypt, signature) {
		return resp, nil, fmt.Errorf("invalid callback signature")
	}

	// 解密回调数据
	plaintext, err := s.decryptCallback(encryptedReq.Encrypt, aesKey)
	if err != nil {
		return resp, nil, fmt.Errorf("failed to decrypt callback: %w", err)
	}

	// 解析解密后的数据
	result, err := s.parseCallbackData(plaintext)
	if err != nil {
		return resp, nil, err
	}

	// 构造加密响应
	encryptedResp, err := s.encryptCallback("success", aesKey, corpID)
	if err != nil {
		// 加密失败时返回明文
		return resp, result, nil
	}

	// 生成响应签名
	respTimestamp := fmt.Sprintf("%d", time.Now().Unix())
	respNonce := nonce
	respSignature := s.generateSignature(token, respTimestamp, respNonce, encryptedResp)

	respBody, _ := json.Marshal(map[string]string{
		"msg_signature": respSignature,
		"timeStamp":     respTimestamp,
		"nonce":         respNonce,
		"encrypt":       encryptedResp,
	})

	resp.Body = string(respBody)
	return resp, result, nil
}

// handlePlainCallback 处理明文回调（兼容旧版本）
func (s *DingTalkSender) handlePlainCallback(rawBody []byte) (domain.CallbackResponse, []*domain.CallbackResult, error) {
	resp := domain.CallbackResponse{
		StatusCode: 200,
		Body:       "success",
	}

	var callbackData struct {
		EventType string `json:"EventType"`
		TaskID    int64  `json:"task_id"`
		CorpID    string `json:"corpid"`
		UserID    string `json:"userid"`
		Status    string `json:"status"`
		ErrCode   int    `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		Timestamp int64  `json:"timestamp"`
	}

	if err := json.Unmarshal(rawBody, &callbackData); err != nil {
		return resp, nil, fmt.Errorf("invalid callback data: %w", err)
	}

	status := constants.CallbackStatusDelivered
	if callbackData.ErrCode != 0 {
		status = constants.CallbackStatusFailed
	}

	reportTime := time.Unix(callbackData.Timestamp/1000, 0)
	if callbackData.Timestamp == 0 {
		reportTime = time.Now()
	}

	return resp, []*domain.CallbackResult{{
		ProviderID:   fmt.Sprintf("%d", callbackData.TaskID),
		Status:       status,
		ErrorCode:    fmt.Sprintf("%d", callbackData.ErrCode),
		ErrorMessage: callbackData.ErrMsg,
		ReportTime:   reportTime,
	}}, nil
}

// parseCallbackData 解析回调数据
func (s *DingTalkSender) parseCallbackData(plaintext []byte) ([]*domain.CallbackResult, error) {
	var callbackData struct {
		EventType string `json:"EventType"`
		TaskID    int64  `json:"task_id"`
		CorpID    string `json:"corpid"`
		UserID    string `json:"userid"`
		Status    string `json:"status"`
		ErrCode   int    `json:"errcode"`
		ErrMsg    string `json:"errmsg"`
		Timestamp int64  `json:"timestamp"`
	}

	if err := json.Unmarshal(plaintext, &callbackData); err != nil {
		return nil, fmt.Errorf("invalid callback data: %w", err)
	}

	// check_url 事件不需要处理
	if callbackData.EventType == "check_url" {
		return nil, nil
	}

	status := constants.CallbackStatusDelivered
	if callbackData.ErrCode != 0 {
		status = constants.CallbackStatusFailed
	}

	reportTime := time.Unix(callbackData.Timestamp/1000, 0)
	if callbackData.Timestamp == 0 {
		reportTime = time.Now()
	}

	return []*domain.CallbackResult{{
		ProviderID:   fmt.Sprintf("%d", callbackData.TaskID),
		Status:       status,
		ErrorCode:    fmt.Sprintf("%d", callbackData.ErrCode),
		ErrorMessage: callbackData.ErrMsg,
		ReportTime:   reportTime,
	}}, nil
}

// verifyCallbackSignature 验证回调签名
func (s *DingTalkSender) verifyCallbackSignature(token, timestamp, nonce, encrypt, signature string) bool {
	return s.generateSignature(token, timestamp, nonce, encrypt) == signature
}

// generateSignature 生成签名
func (s *DingTalkSender) generateSignature(token, timestamp, nonce, encrypt string) string {
	params := []string{token, timestamp, nonce, encrypt}
	sort.Strings(params)
	joined := strings.Join(params, "")

	h := sha1.New()
	h.Write([]byte(joined))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// decryptCallback 解密回调数据
// 钉钉使用 AES-CBC 加密，密钥为 EncodingAESKey + "=" base64解码后的32字节
func (s *DingTalkSender) decryptCallback(encrypted, aesKey string) ([]byte, error) {
	// AESKey = Base64_Decode(EncodingAESKey + "=")
	key, err := base64.StdEncoding.DecodeString(aesKey + "=")
	if err != nil {
		return nil, fmt.Errorf("invalid aes key: %w", err)
	}

	// 解码密文
	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, fmt.Errorf("invalid encrypted data: %w", err)
	}

	// AES-CBC 解密
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	// IV 是密钥的前16字节
	iv := key[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)

	// 去除 PKCS7 填充
	plaintext, err := s.pkcs7Unpad(ciphertext)
	if err != nil {
		return nil, err
	}

	// 钉钉消息格式: random(16bytes) + msg_len(4bytes) + msg + corp_id
	if len(plaintext) < 20 {
		return nil, fmt.Errorf("plaintext too short")
	}

	// 跳过16字节随机数
	plaintext = plaintext[16:]

	// 读取消息长度（4字节大端序）
	msgLen := int(binary.BigEndian.Uint32(plaintext[:4]))
	plaintext = plaintext[4:]

	if msgLen > len(plaintext) {
		return nil, fmt.Errorf("invalid message length")
	}

	// 提取消息内容
	return plaintext[:msgLen], nil
}

// encryptCallback 加密响应数据
func (s *DingTalkSender) encryptCallback(msg, aesKey, corpID string) (string, error) {
	// AESKey = Base64_Decode(EncodingAESKey + "=")
	key, err := base64.StdEncoding.DecodeString(aesKey + "=")
	if err != nil {
		return "", fmt.Errorf("invalid aes key: %w", err)
	}

	// 构造消息: random(16bytes) + msg_len(4bytes) + msg + corp_id
	msgBytes := []byte(msg)
	corpIDBytes := []byte(corpID)

	// 16字节随机数
	random := make([]byte, 16)
	for i := range random {
		random[i] = byte(i)
	}

	// 4字节消息长度（大端序）
	msgLen := make([]byte, 4)
	binary.BigEndian.PutUint32(msgLen, uint32(len(msgBytes)))

	// 拼接
	plaintext := append(random, msgLen...)
	plaintext = append(plaintext, msgBytes...)
	plaintext = append(plaintext, corpIDBytes...)

	// PKCS7 填充
	plaintext = s.pkcs7Pad(plaintext, aes.BlockSize)

	// AES-CBC 加密
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// IV 是密钥的前16字节
	iv := key[:aes.BlockSize]
	mode := cipher.NewCBCEncrypter(block, iv)

	ciphertext := make([]byte, len(plaintext))
	mode.CryptBlocks(ciphertext, plaintext)

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// pkcs7Pad PKCS7填充
func (s *DingTalkSender) pkcs7Pad(data []byte, blockSize int) []byte {
	padding := blockSize - len(data)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(data, padtext...)
}

// pkcs7Unpad PKCS7去填充
func (s *DingTalkSender) pkcs7Unpad(data []byte) ([]byte, error) {
	length := len(data)
	if length == 0 {
		return nil, fmt.Errorf("empty data")
	}
	padding := int(data[length-1])
	if padding > length || padding > aes.BlockSize {
		return nil, fmt.Errorf("invalid padding")
	}
	return data[:length-padding], nil
}
