package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PDF 方案2：先用 Poppler 的 pdftoppm 将每页渲染为 PNG，再逐页走现有 image_path OCR（与图片链路一致，便于换 OCR 厂商）。
const (
	pdfOCRMaxPages = 80
	pdfRenderDPI   = 120 // 降低 DPI 从 144 到 120，提速 20% 以上，且不影响 OCR 准确率
)

// ocrFileWithPDFSupport 非 PDF 直接 OCR；PDF 则先渲染为多张 PNG 再逐页 OCR 并合并文本。
// 优化点：对 PDF 的多页图片并行执行 OCR，显著提升提取速度。
// ctx 用于删除文件时取消进行中的 OCR HTTP；taskID 用于协作式检测 isTaskAborted。
func (s *FileTaskService) ocrFileWithPDFSupport(ctx context.Context, endpoint string, filePath string, taskID string, fileAssetID string) (string, error) {
	ext := strings.ToLower(strings.TrimSpace(filepath.Ext(filePath)))
	if ext != ".pdf" {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		if s.isTaskAborted(taskID) {
			return "", context.Canceled
		}
		return s.callOCR(ctx, endpoint, filePath)
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.isTaskAborted(taskID) {
		return "", context.Canceled
	}

	// 为每个文件建立独立的页面图片存放目录，实实在在供用户查看
	destDir := filepath.Join("data", "files", "pages", fileAssetID)
	pagePaths, err := renderPDFToPersistentPagePNGs(filePath, destDir)
	if err != nil {
		return "", err
	}

	if err := ctx.Err(); err != nil {
		return "", err
	}
	if s.isTaskAborted(taskID) {
		return "", context.Canceled
	}

	total := len(pagePaths)
	if total == 0 {
		return "", fmt.Errorf("PDF 渲染后未得到任何页面图片")
	}

	// 并行控制：每个 PDF 任务最多同时运行 3 个页面 OCR，避免单任务压跨 OCR 服务
	const maxParallelPages = 3
	sem := make(chan struct{}, maxParallelPages)

	results := make([]string, total)
	errs := make([]error, total)

	var wg sync.WaitGroup

	// 单独记录已完成页面数供进度展示
	var completedCount int
	var countMu sync.Mutex

	for i, p := range pagePaths {
		wg.Add(1)
		go func(index int, path string) {
			defer wg.Done()
			sem <- struct{}{} // 获取令牌
			defer func() { <-sem }() // 释放令牌

			if s.isTaskAborted(taskID) {
				errs[index] = context.Canceled
				return
			}
			select {
			case <-ctx.Done():
				errs[index] = ctx.Err()
				return
			default:
			}

			txt, err := s.callOCR(ctx, endpoint, path)
			if err != nil {
				errs[index] = err
				return
			}
			results[index] = strings.TrimSpace(txt)

			// 更新进度
			countMu.Lock()
			completedCount++
			current := completedCount
			countMu.Unlock()
			progressMsg := fmt.Sprintf("正在识别第 %d/%d 页...", current, total)
			s.updateTaskProgress(taskID, 40+(current*25/total), progressMsg)
		}(i, p)
	}
	wg.Wait()

	// 检查是否有错误
	for _, e := range errs {
		if e != nil {
			return "", fmt.Errorf("部分页面识别失败: %w", e)
		}
	}

	var b strings.Builder
	for i, txt := range results {
		if i > 0 {
			b.WriteString("\n\n--- 第 ")
			b.WriteString(strconv.Itoa(i + 1))
			b.WriteString(" 页 ---\n\n")
		}
		b.WriteString(txt)
	}
	return b.String(), nil
}

var pagePPMName = regexp.MustCompile(`^page-(\d+)\.png$`)

// resolvePdftoppmPath 返回可执行的 pdftoppm 路径。IDE/launchd 启动的后端常继承精简 PATH，
// 导致已 brew install poppler 仍报「找不到」；故在 LookPath 失败时尝试常见绝对路径。
func resolvePdftoppmPath() (string, error) {
	if p, err := exec.LookPath("pdftoppm"); err == nil {
		return p, nil
	}
	candidates := []string{
		"/opt/homebrew/bin/pdftoppm", // Apple Silicon Homebrew
		"/usr/local/bin/pdftoppm",   // Intel Mac Homebrew
	}
	for _, p := range candidates {
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", fmt.Errorf("未找到 pdftoppm：请安装 Poppler 并保证在 PATH 中（macOS: brew install poppler；或将 /opt/homebrew/bin 加入启动后端的 PATH）")
}

func renderPDFToPersistentPagePNGs(pdfPath, destDir string) (paths []string, err error) {
	absDestDir, _ := filepath.Abs(destDir)
	if err := os.MkdirAll(absDestDir, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	pdftoppmBin, lookErr := resolvePdftoppmPath()
	if lookErr != nil {
		return nil, fmt.Errorf("PDF 识别需 Poppler：%w", lookErr)
	}

	prefix := filepath.Join(absDestDir, "page")
	
	// 增加 60 秒硬超时，防止 pdftoppm 在处理某些特殊 PDF 时死锁或长时间挂起
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, pdftoppmBin, "-png", "-r", strconv.Itoa(pdfRenderDPI), pdfPath, prefix)
	out, cmdErr := cmd.CombinedOutput()
	if cmdErr != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return nil, fmt.Errorf("PDF 渲染超时 (60s)，文件可能过于复杂或 CPU 负载过高")
		}
		return nil, fmt.Errorf("pdftoppm 失败: %w — %s", cmdErr, strings.TrimSpace(string(out)))
	}

	globs, _ := filepath.Glob(filepath.Join(absDestDir, "page-*.png"))
	if len(globs) == 0 {
		return nil, fmt.Errorf("pdftoppm 未生成 PNG，请确认 PDF 可读")
	}

	sort.Slice(globs, func(i, j int) bool {
		return pageNumFromPPMName(globs[i]) < pageNumFromPPMName(globs[j])
	})

	if len(globs) > pdfOCRMaxPages {
		globs = globs[:pdfOCRMaxPages]
	}

	return globs, nil
}

func pageNumFromPPMName(path string) int {
	base := filepath.Base(path)
	m := pagePPMName.FindStringSubmatch(base)
	if len(m) < 2 {
		return 0
	}
	n, _ := strconv.Atoi(m[1])
	return n
}
