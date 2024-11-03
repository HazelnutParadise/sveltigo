package parse

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"regexp"
	"strings"
)

func renameVariables(content string, index int) string {
	re := regexp.MustCompile(`\blet\s+(\w+)\b`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		parts := strings.Split(match, " ")
		if len(parts) == 2 {
			return parts[0] + " " + parts[1] + fmt.Sprintf("_%d", index)
		}
		return match
	})
}

func scopeCSS(content string, scopeID string) string {
	re := regexp.MustCompile(`(?s)<style.*?>(.*?)</style>`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		cssContent := match[7 : len(match)-8] // 抓取 <style> 標籤內的內容
		// 為每個選擇器加上 [data-scope="scopeID"] 前綴
		cssContent = regexp.MustCompile(`(?m)^([^@\s{}][^{}]*?)\s*{`).ReplaceAllString(cssContent, fmt.Sprintf(`[data-scope="%s"] $1 {`, scopeID))
		return fmt.Sprintf(`<style>%s</style>`, cssContent)
	})
}

func processComponent(content string, index int) (string, error) {
	// 檢查並避免重複的 <script> 標籤
	scriptRe := regexp.MustCompile(`(?s)<script.*?>(.*?)</script>`)
	matches := scriptRe.FindAllStringSubmatch(content, -1)
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple <script> tags detected in component %d", index)
	}

	// 重命名變數以避免衝突
	renamedContent := renameVariables(content, index)
	// 添加 data-scope 標記到最外層元素
	renamedContent = strings.Replace(renamedContent, "<div", fmt.Sprintf(`<div data-scope="%d"`, index), 1)
	// 處理 <style> 標籤以添加範圍屬性
	renamedContent = scopeCSS(renamedContent, fmt.Sprintf("%d", index))
	return renamedContent, nil
}

func AutoParse() (string, error) {
	// 讀取 src 資料夾下的檔案
	files, err := os.ReadDir("./src")
	if err != nil {
		return "", err
	}

	var combinedTemplate strings.Builder

	for i, file := range files {
		if file.IsDir() {
			continue
		}
		content, err := os.ReadFile("./src/" + file.Name())
		if err != nil {
			return "", err
		}
		processedContent, err := processComponent(string(content), i)
		if err != nil {
			return "", err
		}
		combinedTemplate.WriteString(processedContent + "\n")
	}

	// 創建 Go 模板
	tmpl, err := template.New("combined").Parse(combinedTemplate.String())
	if err != nil {
		return "", err
	}

	// 渲染模板
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, nil)
	if err != nil {
		return "", err
	}

	htmlContent := buf.String()

	// 將渲染的 HTML 傳遞給 Svelte 編譯器進行編譯
	svelteResult, err := ParseSvelte(htmlContent)
	if err != nil {
		return "", err
	}

	// 輸出渲染結果
	fmt.Println(svelteResult)

	return svelteResult, nil
}
