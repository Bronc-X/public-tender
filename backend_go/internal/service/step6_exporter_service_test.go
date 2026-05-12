package service

import (
	"archive/zip"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"backend_go/internal/agent"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

func TestNormalizeStep5ChapterBindingsUnwrapsSavedPayload(t *testing.T) {
	raw := map[string]interface{}{
		"bindings": map[string]interface{}{
			"一、投标函": map[string]interface{}{
				"supplement": "写入公司名称",
			},
			"二、授权书": map[string]interface{}{
				"supplement": "写入授权代表",
			},
		},
	}

	bindings := normalizeStep5ChapterBindings(raw)

	if len(bindings) != 2 {
		t.Fatalf("normalizeStep5ChapterBindings returned %d chapters, want 2", len(bindings))
	}
	if _, ok := bindings["bindings"]; ok {
		t.Fatal("normalizeStep5ChapterBindings kept the outer bindings wrapper as a chapter")
	}
	if _, ok := bindings["一、投标函"]; !ok {
		t.Fatal("normalizeStep5ChapterBindings lost chapter 一、投标函")
	}
}

func TestFallbackSlotsRequireBindingContent(t *testing.T) {
	if slots := fallbackSlotsFromChapterBinding("project-empty", "商务标统一模板", nil); len(slots) != 0 {
		t.Fatalf("fallbackSlotsFromChapterBinding returned %d slots for nil binding data, want 0", len(slots))
	}
	if slots := fallbackSlotsFromChapterBinding("project-empty", "一、投标函", map[string]interface{}{}); len(slots) != 0 {
		t.Fatalf("fallbackSlotsFromChapterBinding returned %d slots for empty binding data, want 0", len(slots))
	}
}

func TestBuildWebFormPayloadUsesConfirmedBindingsDeterministically(t *testing.T) {
	db := sqlx.MustConnect("sqlite3", ":memory:")
	t.Cleanup(func() { _ = db.Close() })
	db.MustExec(`
		CREATE TABLE bid_project_actions (
			project_id TEXT,
			action_name TEXT,
			action_status TEXT,
			result_json TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	db.MustExec(`
		INSERT INTO bid_project_actions (project_id, action_name, action_status, result_json)
		VALUES (?, 'step5_chapter_bindings', 'success', ?)
	`, "project-deterministic", `{
		"bindings": {
			"一、投标函": {
				"supplement": "最开始，要写公司名称，四川宏远建筑工程有限公司",
				"resources": [
					{
						"record_name": "汇亮新能源开鲁10MW分散式风力发电项目施工分包合同",
						"category": "业绩",
						"requirement_text": "提供类似项目业绩"
					}
				]
			}
		}
	}`)

	payload, err := NewStep6ExporterService(db).BuildWebFormPayload(context.Background(), "project-deterministic")
	if err != nil {
		t.Fatal(err)
	}
	if len(payload.Slots) != 1 {
		t.Fatalf("slot count = %d, want 1", len(payload.Slots))
	}
	slot := payload.Slots[0]
	if slot.Status != agent.StatusApproved {
		t.Fatalf("slot status = %q, want approved", slot.Status)
	}
	if !strings.Contains(slot.AISuggestedValue, "四川宏远建筑工程有限公司") ||
		!strings.Contains(slot.AISuggestedValue, "汇亮新能源开鲁10MW分散式风力发电项目施工分包合同") {
		t.Fatalf("slot value did not preserve confirmed binding content: %s", slot.AISuggestedValue)
	}
}

func TestExecuteSafeWordExportRejectsEmptySlotsBeforeTemplateAccess(t *testing.T) {
	payload := agent.BidActionList{
		ProjectID: "project-empty",
		Slots:     []agent.BidActionSlot{},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	_, err = NewStep6ExporterService(nil).ExecuteSafeWordExport(context.Background(), string(payloadBytes), "/path/that/does/not/exist.docx")
	if err == nil {
		t.Fatal("expected empty-slot export to fail")
	}
	if !strings.Contains(err.Error(), "no approved slots") {
		t.Fatalf("error = %q, want no approved slots", err.Error())
	}
}

func TestExecuteSafeWordExportWritesProjectScopedFileAndEscapesXML(t *testing.T) {
	exportRoot := t.TempDir()
	t.Setenv("BID_EXPORT_DIR", exportRoot)

	templatePath := filepath.Join(t.TempDir(), "template.docx")
	writeMinimalDocx(t, templatePath, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>{{公司名称}}</w:t></w:r></w:p>
    <w:sectPr/>
  </w:body>
</w:document>`)

	payload := agent.BidActionList{
		ProjectID: "project-123",
		Slots: []agent.BidActionSlot{
			{
				SlotID:           "slot-company",
				ChapterPath:      []string{"一、投标函"},
				SlotContextTitle: "一、投标函",
				TargetField:      "公司名称",
				SlotType:         agent.SlotTypeText,
				AISuggestedValue: "四川宏远 & <施工>",
				Status:           agent.StatusApproved,
			},
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	exportPath, err := NewStep6ExporterService(nil).ExecuteSafeWordExport(context.Background(), string(payloadBytes), templatePath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(exportPath, filepath.Join(exportRoot, "project-123")+string(os.PathSeparator)) {
		t.Fatalf("exportPath = %q, want under project export dir %q", exportPath, exportRoot)
	}

	documentXML := readDocxEntry(t, exportPath, "word/document.xml")
	if !strings.Contains(documentXML, "四川宏远 &amp; &lt;施工&gt;") {
		t.Fatalf("document.xml does not contain XML-escaped slot value: %s", documentXML)
	}
}

func TestExecuteSafeWordExportAppendsSlotValuesWithoutTemplatePlaceholders(t *testing.T) {
	exportRoot := t.TempDir()
	t.Setenv("BID_EXPORT_DIR", exportRoot)

	templatePath := filepath.Join(t.TempDir(), "template.docx")
	writeMinimalDocx(t, templatePath, `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>原始模板正文，没有任何花括号占位符。</w:t></w:r></w:p>
    <w:sectPr/>
  </w:body>
</w:document>`)

	payload := agent.BidActionList{
		ProjectID: "project-append",
		Slots: []agent.BidActionSlot{
			{
				SlotID:           "slot-resource",
				ChapterPath:      []string{"一、投标函"},
				SlotContextTitle: "一、投标函",
				TargetField:      "章节装配数据",
				SlotType:         agent.SlotTypeText,
				AISuggestedValue: "汇亮新能源开鲁10MW分散式风力发电项目施工分包合同",
				Status:           agent.StatusApproved,
			},
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	exportPath, err := NewStep6ExporterService(nil).ExecuteSafeWordExport(context.Background(), string(payloadBytes), templatePath)
	if err != nil {
		t.Fatal(err)
	}

	documentXML := readDocxEntry(t, exportPath, "word/document.xml")
	if !strings.Contains(documentXML, "自动装配资料") || !strings.Contains(documentXML, "汇亮新能源开鲁10MW分散式风力发电项目施工分包合同") {
		t.Fatalf("document.xml does not contain appended slot value: %s", documentXML)
	}
}

func TestExecuteSafeWordExportUsesDefaultTemplateWhenPathEmpty(t *testing.T) {
	exportRoot := t.TempDir()
	t.Setenv("BID_EXPORT_DIR", exportRoot)

	payload := agent.BidActionList{
		ProjectID: "project-default-template",
		Slots: []agent.BidActionSlot{
			{
				SlotID:           "slot-tech",
				ChapterPath:      []string{"一、施工方案"},
				SlotContextTitle: "一、施工方案",
				TargetField:      "施工方案",
				SlotType:         agent.SlotTypeText,
				AISuggestedValue: "按招标文件要求完成天然气安装工程施工。",
				Status:           agent.StatusApproved,
			},
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}

	exportPath, err := NewStep6ExporterService(nil).ExecuteSafeWordExport(context.Background(), string(payloadBytes), "")
	if err != nil {
		t.Fatal(err)
	}
	documentXML := readDocxEntry(t, exportPath, "word/document.xml")
	if !strings.Contains(documentXML, "按招标文件要求完成天然气安装工程施工") {
		t.Fatalf("document.xml does not contain default-template export content: %s", documentXML)
	}
}

func writeMinimalDocx(t *testing.T, path string, documentXML string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	w := zip.NewWriter(f)
	defer w.Close()

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
		"word/document.xml": documentXML,
	}

	for name, content := range entries {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = fw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
}

func readDocxEntry(t *testing.T, path string, entryName string) string {
	t.Helper()
	r, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	for _, f := range r.File {
		if f.Name != entryName {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatal(err)
		}
		return string(data)
	}
	t.Fatalf("entry %s not found in %s", entryName, path)
	return ""
}
