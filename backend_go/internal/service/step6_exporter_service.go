package service

import (
	"archive/zip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"backend_go/internal/agent"

	"github.com/jmoiron/sqlx"
)

// ========= Data Structures for Step 6 (Web Form & Safe Export) =========

// Aliasing for easier transition
type ReviewStatus = agent.SlotStatus
type BidActionSlot = agent.BidActionSlot
type BidActionList = agent.BidActionList

// Step6ExporterService handles the orchestration of Web Form presentation
// and final safe '.docx' text injection.
type Step6ExporterService struct {
	db *sqlx.DB
}

func NewStep6ExporterService(db *sqlx.DB) *Step6ExporterService {
	return &Step6ExporterService{db: db}
}

func (s *Step6ExporterService) BuildWebFormPayload(ctx context.Context, projectID string) (*agent.BidActionList, error) {
	// 1. Fetch step 5 bindings
	var step5BindingsJSON *string
	err := s.db.QueryRow("SELECT result_json FROM bid_project_actions WHERE project_id = ? AND action_name = 'step5_chapter_bindings' AND action_status = 'success' ORDER BY created_at DESC LIMIT 1", projectID).Scan(&step5BindingsJSON)
	if err != nil && err.Error() != "sql: no rows in result set" {
		return nil, fmt.Errorf("failed to fetch step5 bindings: %v", err)
	}

	var rawBindings map[string]interface{}
	var bindings map[string]interface{}
	if step5BindingsJSON != nil {
		_ = json.Unmarshal([]byte(*step5BindingsJSON), &rawBindings)
		bindings = normalizeStep5ChapterBindings(rawBindings)
	}

	if len(bindings) == 0 {
		return nil, fmt.Errorf("no confirmed step 5 chapter bindings found")
	}

	var allSlots []agent.BidActionSlot
	for chapterName, chapterData := range bindings {
		slots := fallbackSlotsFromChapterBinding(projectID, chapterName, chapterData)
		if len(slots) == 0 {
			continue
		}
		allSlots = append(allSlots, slots...)
	}
	if len(allSlots) == 0 {
		return nil, fmt.Errorf("confirmed step 5 chapter bindings contain no exportable content")
	}
	log.Printf("[Step6Exporter] generated %d deterministic slots from confirmed step 5 bindings", len(allSlots))

	// Prepare final aggregated payload
	finalResult := agent.BidActionList{
		ProjectID:        projectID,
		Chapter:          "聚合标书全集装配",
		Slots:            allSlots,
		OriginalMarkdown: "并发多流融合数据隔离完成",
	}

	return &finalResult, nil
}

func normalizeStep5ChapterBindings(raw map[string]interface{}) map[string]interface{} {
	if len(raw) == 0 {
		return nil
	}
	if wrapped, ok := raw["bindings"].(map[string]interface{}); ok {
		return wrapped
	}
	return raw
}

func fallbackSlotsFromChapterBinding(projectID string, chapter string, data interface{}) []agent.BidActionSlot {
	value := formatChapterBindingText(chapter, data)
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return []agent.BidActionSlot{
		{
			SlotID:           "chapter_binding_" + sanitizeFilePart(chapter),
			ChapterPath:      []string{chapter},
			SlotContextTitle: chapter,
			TargetField:      "章节装配数据_" + chapter,
			SlotType:         agent.SlotTypeText,
			AISuggestedValue: value,
			Reason:           "由用户在方案处理确认阶段绑定的资源和补充说明确定。",
			Status:           agent.StatusApproved,
		},
	}
}

func formatChapterBindingText(chapter string, data interface{}) string {
	var b strings.Builder
	writeLine := func(s string) {
		if strings.TrimSpace(s) == "" {
			return
		}
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		b.WriteString(s)
	}

	obj, objOK := data.(map[string]interface{})
	if len(obj) == 0 {
		if data == nil || objOK {
			return ""
		}
		raw, err := json.Marshal(data)
		if err == nil && string(raw) != "null" {
			writeLine("章节：" + chapter)
			writeLine("装配数据：" + string(raw))
		}
		return b.String()
	}

	writeLine("章节：" + chapter)
	contentStart := b.Len()

	if supplement, ok := obj["supplement"].(string); ok && strings.TrimSpace(supplement) != "" {
		writeLine("补充说明：" + supplement)
	}

	resources, _ := obj["resources"].([]interface{})
	if len(resources) > 0 {
		writeLine("已绑定资源：")
		for _, item := range resources {
			resource, _ := item.(map[string]interface{})
			name := stringValue(resource["record_name"])
			if name == "" {
				name = stringValue(resource["name"])
			}
			category := stringValue(resource["category"])
			requirement := stringValue(resource["requirement_text"])
			line := "- " + name
			if category != "" {
				line += "（" + category + "）"
			}
			if requirement != "" {
				line += "：对应要求：" + requirement
			}
			writeLine(line)
		}
	}

	if b.Len() == contentStart {
		return ""
	}
	return b.String()
}

func stringValue(v interface{}) string {
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

// ExecuteSafeWordExport finalizes the Step 6 pipeline:
// Receives the fully approved BidActionList (where Status == "approved"),
// replaces variables cleanly in the template docx, and avoids Native Comments.
func (s *Step6ExporterService) ExecuteSafeWordExport(ctx context.Context, approvedPayload string, templateFilePath string) (string, error) {
	return s.ExecuteSafeWordExportToDir(ctx, approvedPayload, templateFilePath, "")
}

// ExecuteSafeWordExportToDir is the same safe Step 6 export flow, with an explicit
// export root for callers that must keep their files in a separate project namespace.
func (s *Step6ExporterService) ExecuteSafeWordExportToDir(ctx context.Context, approvedPayload string, templateFilePath string, exportRoot string) (string, error) {
	var payload BidActionList
	if err := json.Unmarshal([]byte(approvedPayload), &payload); err != nil {
		return "", fmt.Errorf("failed to parse approved payload: %v", err)
	}
	if len(payload.Slots) == 0 {
		return "", fmt.Errorf("no approved slots to export")
	}

	// 1. Verify all slots are approved to prevent exporting unverified AI data.
	replacements := make(map[string]string)
	arrayBlocks := make(map[string]string)
	imageBlocks := make(map[string]string)
	appendixBlocks := make([]appendixBlock, 0, len(payload.Slots))

	for _, slot := range payload.Slots {
		if slot.Status != agent.StatusApproved {
			return "", fmt.Errorf("cannot export doc: slot '%s' is not approved", slot.SlotID)
		}

		if slot.SlotType == "image" {
			imageBlocks[slot.TargetField] = slot.AISuggestedValue
		} else if slot.SlotType == agent.SlotTypePersonnelTable || slot.SlotType == agent.SlotTypePerformance ||
			slot.SlotType == agent.SlotTypeCompanyProfile || slot.SlotType == agent.SlotTypeCertificateList {
			arrayBlocks[slot.TargetField] = slot.AISuggestedValue
		} else {
			// Expecting the Word template to have {{Field Name}} as placeholders
			placeholder := fmt.Sprintf("{{%s}}", slot.TargetField)
			replacements[placeholder] = xmlEscapeText(slot.AISuggestedValue)
		}
		appendixBlocks = append(appendixBlocks, appendixBlock{
			Title: slot.SlotContextTitle,
			Field: slot.TargetField,
			Value: slot.AISuggestedValue,
		})
	}

	// 2. Load Word template & Run Native Replacement
	exportPath, err := buildStep6ExportPathWithRoot(payload.ProjectID, exportRoot)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(templateFilePath) == "" {
		defaultTemplate, err := createDefaultStep6Template()
		if err != nil {
			return "", err
		}
		defer os.Remove(defaultTemplate)
		templateFilePath = defaultTemplate
	}
	err = s.replaceDocxPlaceholders(templateFilePath, exportPath, replacements, arrayBlocks, imageBlocks, appendixBlocks)
	if err != nil {
		return "", fmt.Errorf("failed to process document replacement: %v", err)
	}

	// 3. Output safe & clean path
	return exportPath, nil
}

func createDefaultStep6Template() (string, error) {
	f, err := os.CreateTemp("", "step6-default-template-*.docx")
	if err != nil {
		return "", fmt.Errorf("failed to create default template: %v", err)
	}
	path := f.Name()
	w := zip.NewWriter(f)
	entries := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>
</Relationships>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>技术标最终文档</w:t></w:r></w:p>
    <w:sectPr/>
  </w:body>
</w:document>`,
	}
	for name, content := range entries {
		fw, err := w.Create(name)
		if err != nil {
			_ = w.Close()
			_ = f.Close()
			_ = os.Remove(path)
			return "", fmt.Errorf("failed to create default template entry: %v", err)
		}
		if _, err := fw.Write([]byte(content)); err != nil {
			_ = w.Close()
			_ = f.Close()
			_ = os.Remove(path)
			return "", fmt.Errorf("failed to write default template entry: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf("failed to finalize default template: %v", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", fmt.Errorf("failed to close default template: %v", err)
	}
	return path, nil
}

// Dummy 1x1 transparent PNG for fallback
const dummyPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8/5+hHgAHggJ/PchI7wAAAABJRU5ErkJggg=="

type appendixBlock struct {
	Title string
	Field string
	Value string
}

// replaceDocxPlaceholders directly edits the XML payload of the docx natively via ZIP,
// guaranteeing zero disruption to the intricate table formats and removing the need for a Python worker.
func (s *Step6ExporterService) replaceDocxPlaceholders(srcPath, dstPath string, replacements map[string]string, arrayBlocks map[string]string, imageBlocks map[string]string, appendixBlocks []appendixBlock) error {
	r, err := zip.OpenReader(srcPath)
	if err != nil {
		return err
	}
	defer r.Close()

	destFile, err := os.Create(dstPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	w := zip.NewWriter(destFile)
	defer w.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}

		if f.Name == "[Content_Types].xml" {
			content, _ := io.ReadAll(rc)
			rc.Close()
			strContent := string(content)
			if !strings.Contains(strContent, `Extension="png"`) {
				strContent = strings.Replace(strContent, "</Types>", `<Default Extension="png" ContentType="image/png"/></Types>`, 1)
			}
			fw, _ := w.Create(f.Name)
			fw.Write([]byte(strContent))
			continue
		}

		if f.Name == "word/_rels/document.xml.rels" {
			content, _ := io.ReadAll(rc)
			rc.Close()
			strContent := string(content)
			idx := 1
			for range imageBlocks {
				relStr := fmt.Sprintf(`<Relationship Id="rId_custom_img_%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/runtime_img_%d.png"/>`, idx, idx)
				strContent = strings.Replace(strContent, "</Relationships>", relStr+"</Relationships>", 1)
				idx++
			}
			fw, _ := w.Create(f.Name)
			fw.Write([]byte(strContent))
			continue
		}

		// Only string-replace in core text-bearing XMLs. Keep all binary/configs untouched.
		if f.Name == "word/document.xml" || strings.HasPrefix(f.Name, "word/header") || strings.HasPrefix(f.Name, "word/footer") {
			content, err := io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return err
			}

			strContent := string(content)

			// Handle Array blocks like {{#personnel_table}}...{{/personnel_table}}
			if len(arrayBlocks) > 0 {
				strContent = ProcessArrayBlocks(strContent, arrayBlocks)
			}

			// Naive replacement. Note: in production, run-tag splitting of placeholders
			// <w:t>{</w:t><w:t>{xxx}}</w:t> requires removing internal tags first.
			strContent = CleanDocxTags(strContent)
			for k, v := range replacements {
				strContent = strings.ReplaceAll(strContent, k, v)
			}

			// Handle DrawingML Injection for Images
			idx := 1
			for targetField := range imageBlocks {
				placeholder := fmt.Sprintf("{{%s}}", targetField)
				// cx/cy are in EMUs (1905000 approx 2x2 inches)
				drawingML := fmt.Sprintf(`<w:drawing><wp:inline distT="0" distB="0" distL="0" distR="0"><wp:extent cx="1905000" cy="1905000"/><wp:effectExtent l="0" t="0" r="0" b="0"/><wp:docPr id="%d" name="Picture %d"/><wp:cNvGraphicFramePr><a:graphicFrameLocks xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" noChangeAspect="1"/></wp:cNvGraphicFramePr><a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"><pic:nvPicPr><pic:cNvPr id="%d" name="runtime_img_%d.png"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill><a:blip r:embed="rId_custom_img_%d"/><a:stretch><a:fillRect/></a:stretch></pic:blipFill><pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="1905000" cy="1905000"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></pic:spPr></pic:pic></a:graphicData></a:graphic></wp:inline></w:drawing>`, idx, idx, idx, idx, idx)
				replacementXML := fmt.Sprintf(`</w:t></w:r><w:r>%s</w:r><w:r><w:t xml:space="preserve">`, drawingML)
				strContent = strings.ReplaceAll(strContent, placeholder, replacementXML)
				idx++
			}

			if f.Name == "word/document.xml" && len(appendixBlocks) > 0 {
				strContent = appendStep6Appendix(strContent, appendixBlocks)
			}

			fw, err := w.Create(f.Name)
			if err != nil {
				return err
			}
			if _, err = fw.Write([]byte(strContent)); err != nil {
				return err
			}
		} else {
			// Copy image/binary/schema unmodified
			fw, err := w.Create(f.Name)
			if err != nil {
				rc.Close()
				return err
			}
			if _, err = io.Copy(fw, rc); err != nil {
				rc.Close()
				return err
			}
			rc.Close()
		}
	}

	// Finally, inject the physical raw images into the zip
	idx := 1
	for _, imgPath := range imageBlocks {
		imgBytes, err := os.ReadFile(imgPath)
		if err != nil {
			// fallback to dummy transparent png if we cant find the user's path
			imgBytes, _ = base64.StdEncoding.DecodeString(dummyPNG)
		}
		fw, err := w.Create(fmt.Sprintf("word/media/runtime_img_%d.png", idx))
		if err == nil {
			fw.Write(imgBytes)
		}
		idx++
	}

	return nil
}

func buildStep6ExportPath(projectID string) (string, error) {
	return buildStep6ExportPathWithRoot(projectID, "")
}

func buildStep6ExportPathWithRoot(projectID string, exportRoot string) (string, error) {
	root := strings.TrimSpace(exportRoot)
	if root == "" {
		root = os.Getenv("BID_EXPORT_DIR")
	}
	if root == "" {
		root = filepath.Join("data", "exports", "bid_projects")
	}
	projectDir := filepath.Join(root, sanitizeFilePart(projectID))
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create export directory: %v", err)
	}
	fileName := fmt.Sprintf("export_%s_%s.docx", sanitizeFilePart(projectID), time.Now().Format("20060102150405"))
	return filepath.Join(projectDir, fileName), nil
}

func sanitizeFilePart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return strings.Trim(b.String(), "_")
}

func xmlEscapeText(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"'", "&apos;",
	)
	return replacer.Replace(value)
}

func appendStep6Appendix(documentXML string, blocks []appendixBlock) string {
	appendixXML := buildStep6AppendixXML(blocks)
	if appendixXML == "" {
		return documentXML
	}
	if idx := strings.LastIndex(documentXML, "<w:sectPr"); idx >= 0 {
		return documentXML[:idx] + appendixXML + documentXML[idx:]
	}
	if idx := strings.LastIndex(documentXML, "</w:body>"); idx >= 0 {
		return documentXML[:idx] + appendixXML + documentXML[idx:]
	}
	return documentXML + appendixXML
}

func buildStep6AppendixXML(blocks []appendixBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<w:p><w:r><w:t>自动装配资料</w:t></w:r></w:p>`)
	for _, block := range blocks {
		if strings.TrimSpace(block.Value) == "" {
			continue
		}
		title := block.Title
		if title == "" {
			title = block.Field
		}
		if title != "" {
			b.WriteString(`<w:p><w:r><w:t>`)
			b.WriteString(xmlEscapeText(title))
			b.WriteString(`</w:t></w:r></w:p>`)
		}
		for _, line := range strings.Split(block.Value, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			b.WriteString(`<w:p><w:r><w:t>`)
			b.WriteString(xmlEscapeText(line))
			b.WriteString(`</w:t></w:r></w:p>`)
		}
	}
	return b.String()
}
