package service

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	docmind "github.com/alibabacloud-go/docmind-api-20220711/client"
	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	openapiutil "github.com/alibabacloud-go/darabonba-openapi/v2/utils"
	"github.com/alibabacloud-go/tea/dara"
)

type DocMindParseService struct {
	settings *SettingsService
}

func NewDocMindParseService(s *SettingsService) *DocMindParseService {
	return &DocMindParseService{settings: s}
}

func (s *DocMindParseService) Enabled() bool {
	if s == nil || s.settings == nil {
		return false
	}
	ak := strings.TrimSpace(s.settings.GetSetting("doc_parser_access_key"))
	sk := strings.TrimSpace(s.settings.GetSetting("doc_parser_access_secret"))
	return ak != "" && sk != ""
}

func regionFromEndpoint(endpoint string) string {
	endpoint = strings.TrimPrefix(strings.TrimPrefix(strings.TrimSpace(endpoint), "https://"), "http://")
	parts := strings.Split(endpoint, ".")
	for _, p := range parts {
		if strings.HasPrefix(p, "cn-") || strings.HasPrefix(p, "ap-") ||
			strings.HasPrefix(p, "eu-") || strings.HasPrefix(p, "us-") {
			return p
		}
	}
	return "cn-hangzhou"
}

// postOSSUploadForDocmind 将文件 POST 到 AuthorizeFileUpload 返回的 OSS（与官方 SDK _postOSSObject 等价，仅用公开 API）。
func postOSSUploadForDocmind(cli *docmind.Client, bucketName *string, form map[string]interface{}, runtime *dara.RuntimeOptions) (map[string]interface{}, error) {
	_runtime := dara.NewRuntimeObject(map[string]interface{}{
		"key":            dara.ToString(dara.Default(dara.StringValue(runtime.Key), dara.StringValue(cli.Key))),
		"cert":           dara.ToString(dara.Default(dara.StringValue(runtime.Cert), dara.StringValue(cli.Cert))),
		"ca":             dara.ToString(dara.Default(dara.StringValue(runtime.Ca), dara.StringValue(cli.Ca))),
		"readTimeout":    dara.ForceInt(dara.Default(dara.IntValue(runtime.ReadTimeout), dara.IntValue(cli.ReadTimeout))),
		"connectTimeout": dara.ForceInt(dara.Default(dara.IntValue(runtime.ConnectTimeout), dara.IntValue(cli.ConnectTimeout))),
		"httpProxy":      dara.ToString(dara.Default(dara.StringValue(runtime.HttpProxy), dara.StringValue(cli.HttpProxy))),
		"httpsProxy":     dara.ToString(dara.Default(dara.StringValue(runtime.HttpsProxy), dara.StringValue(cli.HttpsProxy))),
		"noProxy":        dara.ToString(dara.Default(dara.StringValue(runtime.NoProxy), dara.StringValue(cli.NoProxy))),
		"socks5Proxy":    dara.ToString(dara.Default(dara.StringValue(runtime.Socks5Proxy), dara.StringValue(cli.Socks5Proxy))),
		"socks5NetWork":  dara.ToString(dara.Default(dara.StringValue(runtime.Socks5NetWork), dara.StringValue(cli.Socks5NetWork))),
		"maxIdleConns":   dara.ForceInt(dara.Default(dara.IntValue(runtime.MaxIdleConns), dara.IntValue(cli.MaxIdleConns))),
		"retryOptions":   cli.RetryOptions,
		"ignoreSSL":      dara.ForceBoolean(dara.Default(dara.BoolValue(runtime.IgnoreSSL), false)),
		"tlsMinVersion":  dara.StringValue(cli.TlsMinVersion),
	})
	request_ := dara.NewRequest()
	boundary := dara.GetBoundary()
	request_.Protocol = dara.String("HTTPS")
	request_.Method = dara.String("POST")
	request_.Pathname = dara.String("/")
	request_.Headers = map[string]*string{
		"host":       dara.String(dara.ToString(form["host"])),
		"date":       openapiutil.GetDateUTCString(),
		"user-agent": openapiutil.GetUserAgent(dara.String("")),
	}
	request_.Headers["content-type"] = dara.String("multipart/form-data; boundary=" + boundary)
	request_.Body = dara.ToFileForm(form, boundary)
	response_, err := dara.DoRequest(request_, _runtime)
	if err != nil {
		return nil, err
	}
	var respMap map[string]interface{}
	bodyStr, err := dara.ReadAsString(response_.Body)
	if err != nil {
		return nil, err
	}
	if dara.IntValue(response_.StatusCode) >= 400 && dara.IntValue(response_.StatusCode) < 600 {
		respMap = dara.ParseXml(bodyStr, nil)
		errMap := dara.ToMap(respMap["Error"])
		return nil, &openapi.ClientError{
			Code:    dara.String(dara.ToString(errMap["Code"])),
			Message: dara.String(dara.ToString(errMap["Message"])),
			Data: map[string]interface{}{
				"httpCode":  dara.IntValue(response_.StatusCode),
				"requestId": dara.ToString(errMap["RequestId"]),
				"hostId":    dara.ToString(errMap["HostId"]),
			},
		}
	}
	respMap = dara.ParseXml(bodyStr, nil)
	out := make(map[string]interface{})
	if err := dara.Convert(dara.ToMap(respMap), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// submitDocParserJobMinimal 走 AuthorizeFileUpload → OSS → SubmitDocParserJobWithOptions，且**不**经过 Convert，
// 避免 Advance 请求经 JSON 往返后带上非法 Option（NotSupportOptionType）。
func submitDocParserJobMinimal(cli *docmind.Client, file *os.File, fileBaseName, fileExt string, rt *dara.RuntimeOptions) (*docmind.SubmitDocParserJobResponse, error) {
	if dara.IsNil(cli.Credential) {
		return nil, &openapi.ClientError{
			Code:    dara.String("InvalidCredentials"),
			Message: dara.String("文档解析客户端未配置有效凭证"),
		}
	}
	credentialModel, err := cli.Credential.GetCredential()
	if err != nil {
		return nil, err
	}
	accessKeyId := dara.StringValue(credentialModel.AccessKeyId)
	accessKeySecret := dara.StringValue(credentialModel.AccessKeySecret)
	securityToken := dara.StringValue(credentialModel.SecurityToken)
	credentialType := dara.StringValue(credentialModel.Type)
	openPlatformEndpoint := dara.StringValue(cli.OpenPlatformEndpoint)
	if dara.IsNil(dara.String(openPlatformEndpoint)) || openPlatformEndpoint == "" {
		openPlatformEndpoint = "openplatform.aliyuncs.com"
	}
	if dara.IsNil(dara.String(credentialType)) {
		credentialType = "access_key"
	}
	authConfig := &openapiutil.Config{
		AccessKeyId:     dara.String(accessKeyId),
		AccessKeySecret: dara.String(accessKeySecret),
		SecurityToken:   dara.String(securityToken),
		Type:            dara.String(credentialType),
		Endpoint:        dara.String(openPlatformEndpoint),
		Protocol:        cli.Protocol,
		RegionId:        cli.RegionId,
	}
	authClient, err := openapi.NewClient(authConfig)
	if err != nil {
		return nil, err
	}
	authReq := &openapiutil.OpenApiRequest{
		Query: openapiutil.Query(map[string]*string{
			"Product":  dara.String("docmind-api"),
			"RegionId": cli.RegionId,
		}),
	}
	authParams := &openapiutil.Params{
		Action:      dara.String("AuthorizeFileUpload"),
		Version:     dara.String("2019-12-19"),
		Protocol:    dara.String("HTTPS"),
		Pathname:    dara.String("/"),
		Method:      dara.String("GET"),
		AuthType:    dara.String("AK"),
		Style:       dara.String("RPC"),
		ReqBodyType: dara.String("formData"),
		BodyType:    dara.String("json"),
	}
	authResponse, err := authClient.CallApi(authParams, authReq, rt)
	if err != nil {
		return nil, err
	}
	tmpBody := dara.ToMap(authResponse["body"])
	useAccelerate := dara.ForceBoolean(tmpBody["UseAccelerate"])
	authResponseBody := openapiutil.StringifyMapValue(tmpBody)
	fileObj := &dara.FileField{
		Filename:    authResponseBody["ObjectKey"],
		Content:     file,
		ContentType: dara.String(""),
	}
	ossHeader := map[string]interface{}{
		"host":                  dara.StringValue(authResponseBody["Bucket"]) + "." + dara.StringValue(openapiutil.GetEndpoint(authResponseBody["Endpoint"], dara.Bool(useAccelerate), cli.EndpointType)),
		"OSSAccessKeyId":        dara.StringValue(authResponseBody["AccessKeyId"]),
		"policy":                dara.StringValue(authResponseBody["EncodedPolicy"]),
		"Signature":             dara.StringValue(authResponseBody["Signature"]),
		"key":                   dara.StringValue(authResponseBody["ObjectKey"]),
		"file":                  fileObj,
		"success_action_status": "201",
	}
	if _, err := postOSSUploadForDocmind(cli, authResponseBody["Bucket"], ossHeader, rt); err != nil {
		return nil, err
	}
	fileURL := "http://" + dara.StringValue(authResponseBody["Bucket"]) + "." + dara.StringValue(authResponseBody["Endpoint"]) + "/" + dara.StringValue(authResponseBody["ObjectKey"])
	submit := &docmind.SubmitDocParserJobRequest{}
	submit.FileName = dara.String(fileBaseName)
	submit.FileNameExtension = dara.String(fileExt)
	submit.FileUrl = dara.String(fileURL)
	return cli.SubmitDocParserJobWithOptions(submit, rt)
}

// ParseLocalFile 提交本地文件到阿里云文档智能解析，轮询完成后返回 Markdown 与纯文本（与 Markdown 相同或后续可拆分）。
func (s *DocMindParseService) ParseLocalFile(absPath, displayName string) (markdown string, plain string, err error) {
	if s == nil || s.settings == nil {
		return "", "", fmt.Errorf("文档解析服务未初始化")
	}
	ak := strings.TrimSpace(s.settings.GetSetting("doc_parser_access_key"))
	sk := strings.TrimSpace(s.settings.GetSetting("doc_parser_access_secret"))
	if ak == "" || sk == "" {
		return "", "", fmt.Errorf("未配置 doc_parser_access_key / doc_parser_access_secret，请在系统设置中填写阿里云文档解析凭证")
	}
	ep := strings.TrimSpace(s.settings.GetSetting("doc_parser_endpoint"))
	if ep == "" {
		ep = "docmind-api.cn-hangzhou.aliyuncs.com"
	}
	ep = strings.TrimPrefix(strings.TrimPrefix(ep, "https://"), "http://")
	region := regionFromEndpoint(ep)

	cfg := &openapiutil.Config{
		AccessKeyId:     dara.String(ak),
		AccessKeySecret: dara.String(sk),
		Endpoint:        dara.String(ep),
		RegionId:        dara.String(region),
	}
	cli, err := docmind.NewClient(cfg)
	if err != nil {
		return "", "", err
	}

	f, err := os.Open(absPath)
	if err != nil {
		return "", "", err
	}
	defer f.Close()

	base := filepath.Base(displayName)
	if base == "" || base == "." {
		base = filepath.Base(absPath)
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(base)), ".")
	if ext == "" {
		ext = strings.TrimPrefix(strings.ToLower(filepath.Ext(absPath)), ".")
	}

	rt := &dara.RuntimeOptions{}
	rt.ReadTimeout = dara.Int(300000)
	rt.ConnectTimeout = dara.Int(60000)

	if _, err = f.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}
	submitResp, err := submitDocParserJobMinimal(cli, f, base, ext, rt)
	if err != nil {
		return "", "", err
	}
	if submitResp == nil || submitResp.Body == nil {
		return "", "", fmt.Errorf("文档解析提交失败：空响应")
	}
	code := dara.StringValue(submitResp.Body.Code)
	if code != "" && code != "200" && !strings.EqualFold(code, "success") {
		return "", "", fmt.Errorf("文档解析提交失败：%s %s", code, dara.StringValue(submitResp.Body.Message))
	}
	if submitResp.Body.Data == nil || submitResp.Body.Data.Id == nil {
		return "", "", fmt.Errorf("文档解析未返回任务 Id")
	}
	jobID := dara.StringValue(submitResp.Body.Data.Id)

	done := false
	for i := 0; i < 120; i++ {
		time.Sleep(3 * time.Second)
		qresp, qerr := cli.QueryDocParserStatus(&docmind.QueryDocParserStatusRequest{Id: dara.String(jobID)})
		if qerr != nil || qresp == nil || qresp.Body == nil || qresp.Body.Data == nil || qresp.Body.Data.Status == nil {
			continue
		}
		st := strings.ToLower(strings.TrimSpace(dara.StringValue(qresp.Body.Data.Status)))
		if st == "success" {
			done = true
			break
		}
		if strings.Contains(st, "fail") {
			return "", "", fmt.Errorf("阿里云文档解析失败：%s", st)
		}
	}
	if !done {
		return "", "", fmt.Errorf("文档解析超时，请稍后重试")
	}

	md, err := fetchDocParserResultAllMarkdown(cli, jobID, rt)
	if err != nil {
		return "", "", err
	}
	return md, md, nil
}

// fetchDocParserResultAllMarkdown 大模型版 GetDocParserResult 要求 LayoutNum 必填，结果按 Layout 分页；逐页拉取并拼接 Markdown。
func fetchDocParserResultAllMarkdown(cli *docmind.Client, jobID string, rt *dara.RuntimeOptions) (string, error) {
	const (
		maxPages       = 500
		layoutStepSize = int32(1000)
	)
	var out strings.Builder
	for page := int32(1); page <= maxPages; page++ {
		p, step := page, layoutStepSize
		req := &docmind.GetDocParserResultRequest{
			Id:             dara.String(jobID),
			LayoutNum:      &p,
			LayoutStepSize: &step,
		}
		gresp, err := cli.GetDocParserResultWithOptions(req, rt)
		if err != nil {
			return "", err
		}
		if gresp == nil || gresp.Body == nil {
			return "", fmt.Errorf("获取解析结果失败：空响应")
		}
		gcode := dara.StringValue(gresp.Body.Code)
		msg := dara.StringValue(gresp.Body.Message)
		if gcode != "" && gcode != "200" && !strings.EqualFold(gcode, "success") {
			if page > 1 && (strings.Contains(strings.ToLower(msg), "layout") || strings.Contains(strings.ToLower(gcode), "layout")) {
				break
			}
			return "", fmt.Errorf("获取解析结果失败：%s %s", gcode, msg)
		}
		data := gresp.Body.Data
		chunk := strings.TrimSpace(extractMarkdownFromDocParserData(data))
		layouts, hasLayouts := data["Layouts"].([]interface{})

		if !hasLayouts {
			if chunk != "" {
				out.WriteString(chunk)
			}
			break
		}
		if len(layouts) == 0 {
			if page == 1 && chunk == "" {
				return "", fmt.Errorf("解析结果为空，请检查阿里云控制台文档解析能力与返回字段")
			}
			break
		}
		if chunk != "" {
			if out.Len() > 0 {
				out.WriteString("\n\n")
			}
			out.WriteString(chunk)
		}
		if len(layouts) < int(layoutStepSize) {
			break
		}
	}
	s := strings.TrimSpace(out.String())
	if s == "" {
		return "", fmt.Errorf("解析结果为空，请检查阿里云控制台文档解析能力与返回字段")
	}
	return s, nil
}

// docMindMarkdownKeys 阿里云文档解析 GetDocParserResult 常见字段：整页或单块 Markdown。
// 新版 Layout 条目常用 markdownContent（camelCase），旧示例为 Markdown / Content。
var docMindMarkdownKeys = []string{
	"Markdown", "markdown", "markdownContent", "Md", "md",
	"Content", "content", "Text", "text", "TextContent",
}

func stringField(m map[string]interface{}, keys []string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

// MarkdownFromDocMindStoredJSON converts stored DocMind JSON payload to readable markdown.
func MarkdownFromDocMindStoredJSON(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	s = strings.TrimPrefix(s, "\ufeff")
	if s == "" {
		return "", fmt.Errorf("empty content")
	}
	var generic interface{}
	if err := json.Unmarshal([]byte(s), &generic); err != nil {
		return "", fmt.Errorf("invalid docmind json: %w", err)
	}
	var data map[string]interface{}
	switch v := generic.(type) {
	case map[string]interface{}:
		data = v
	case []interface{}:
		data = map[string]interface{}{"Layouts": v}
	default:
		return "", fmt.Errorf("unsupported docmind json root type")
	}
	md := strings.TrimSpace(extractMarkdownFromDocParserData(data))
	if md == "" {
		for _, k := range []string{"Data", "data", "Body", "body"} {
			if inner, ok := data[k].(map[string]interface{}); ok {
				md = strings.TrimSpace(extractMarkdownFromDocParserData(inner))
				if md != "" {
					break
				}
			}
		}
	}
	if md == "" {
		return "", fmt.Errorf("unable to extract markdown from docmind json")
	}
	return md, nil
}

func extractMarkdownFromDocParserData(data map[string]interface{}) string {
	if data == nil {
		return ""
	}
	if s := stringField(data, docMindMarkdownKeys); s != "" {
		return s
	}
	layoutsRaw, ok := data["Layouts"]
	if !ok {
		layoutsRaw = data["layouts"]
	}
	layouts, ok := layoutsRaw.([]interface{})
	if !ok || len(layouts) == 0 {
		return ""
	}
	var b strings.Builder
	for _, item := range layouts {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		if s := stringField(m, docMindMarkdownKeys); s != "" {
			b.WriteString(s)
			if !strings.HasSuffix(s, "\n") {
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}
	}
	if b.Len() > 0 {
		return strings.TrimSpace(b.String())
	}
	return ""
}
