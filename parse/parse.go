package parse

import (
	"bytes"
	"crypto/md5"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

func renameIdentifiers(content string, componentName string) (string, map[string]string) {
	// 創建變數映射表
	varMap := make(map[string]string)

	// 先找出所有需要重命名的變數和函數
	re := regexp.MustCompile(`\b(let|const|function|var)\s+([a-zA-Z_][a-zA-Z0-9_]*)\b`)
	matches := re.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		oldName := match[2]
		newName := fmt.Sprintf("%s_%s", componentName, oldName)
		varMap[oldName] = newName
	}

	// 保護 Svelte 表達式
	exprMap := make(map[string]string)
	content = regexp.MustCompile(`{[^}]+}`).ReplaceAllStringFunc(content, func(match string) string {
		key := fmt.Sprintf("__EXPR_%x__", md5.Sum([]byte(match)))
		exprMap[key] = match
		return key
	})

	// 重命名變數和函數宣告
	content = re.ReplaceAllStringFunc(content, func(match string) string {
		parts := strings.Fields(match)
		if len(parts) >= 2 {
			oldName := parts[len(parts)-1]
			return fmt.Sprintf("%s %s", parts[0], varMap[oldName])
		}
		return match
	})

	// 重命名變數使用處
	for oldName, newName := range varMap {
		content = regexp.MustCompile(`\b`+oldName+`\b`).ReplaceAllString(content, newName)
	}

	// 恢復並更新 Svelte 表達式
	for key, expr := range exprMap {
		for oldName, newName := range varMap {
			expr = regexp.MustCompile(`\b`+oldName+`\b`).ReplaceAllString(expr, newName)
		}
		content = strings.ReplaceAll(content, key, expr)
	}

	return content, varMap
}

func scopeCSS(content string, componentName string) string {
	re := regexp.MustCompile(`(?s)<style>(.*?)</style>`)
	return re.ReplaceAllStringFunc(content, func(match string) string {
		cssContent := re.FindStringSubmatch(match)[1]
		// 為每個選擇器加上組件類名
		cssContent = regexp.MustCompile(`([^}]*){`).ReplaceAllStringFunc(cssContent, func(selector string) string {
			selector = strings.TrimSpace(selector[:len(selector)-1])
			if selector != "" && !strings.HasPrefix(selector, "@") {
				selectors := strings.Split(selector, ",")
				for i, s := range selectors {
					s = strings.TrimSpace(s)
					if !strings.HasPrefix(s, "."+componentName) {
						// 處理多層選擇器
						subSelectors := strings.Split(s, " ")
						for j, sub := range subSelectors {
							if !strings.HasPrefix(sub, ":") && !strings.HasPrefix(sub, "."+componentName) {
								subSelectors[j] = "." + componentName + " " + sub
							}
						}
						selectors[i] = strings.Join(subSelectors, " ")
					}
				}
				selector = strings.Join(selectors, ", ")
			}
			return selector + " {"
		})
		return fmt.Sprintf("<style>\n%s\n</style>", cssContent)
	})
}

func addComponentClass(content string, componentName string) string {
	htmlRe := regexp.MustCompile(`<(div|span|section|article|header|footer|main|nav|aside|p|h[1-6]|ul|ol|li|table|tr|td|th)([^>]*?)>`)
	return htmlRe.ReplaceAllStringFunc(content, func(match string) string {
		if strings.Contains(match, `class=`) {
			return regexp.MustCompile(`class="([^"]*)"`).ReplaceAllString(
				match, fmt.Sprintf(`class="$1 %s"`, componentName))
		}
		// 在標籤結尾前添加 class 屬性
		tagEnd := strings.LastIndex(match, ">")
		if tagEnd == -1 {
			return match
		}
		return match[:tagEnd] + fmt.Sprintf(` class="%s"`, componentName) + match[tagEnd:]
	})
}

func mergeScriptsAndStyles(contents []string) (string, string) {
	var mergedScript strings.Builder
	var mergedStyle strings.Builder

	mergedScript.WriteString("<script>\n")
	for _, content := range contents {
		// 移除 Go template 相關的標記
		content = regexp.MustCompile(`{{define\s+"[^"]+"}}`).ReplaceAllString(content, "")
		content = regexp.MustCompile(`{{end}}`).ReplaceAllString(content, "")

		componentName := strings.TrimSuffix(filepath.Base(content), ".svelte")
		scriptRe := regexp.MustCompile(`(?s)<script>(.*?)</script>`)
		matches := scriptRe.FindAllStringSubmatch(content, -1)
		if len(matches) > 0 {
			for _, match := range matches {
				script := strings.TrimSpace(match[1])
				if script != "" {
					mergedScript.WriteString(fmt.Sprintf("// %s\n", componentName))
					mergedScript.WriteString(script + "\n\n")
				}
			}
		}
	}
	mergedScript.WriteString("</script>\n")

	mergedStyle.WriteString("<style>\n")
	for _, content := range contents {
		// 移除 Go template 相關的標記
		content = regexp.MustCompile(`{{define\s+"[^"]+"}}`).ReplaceAllString(content, "")
		content = regexp.MustCompile(`{{end}}`).ReplaceAllString(content, "")

		componentName := strings.TrimSuffix(filepath.Base(content), ".svelte")
		styleRe := regexp.MustCompile(`(?s)<style>(.*?)</style>`)
		matches := styleRe.FindAllStringSubmatch(content, -1)
		if len(matches) > 0 {
			for _, match := range matches {
				style := strings.TrimSpace(match[1])
				if style != "" {
					mergedStyle.WriteString(fmt.Sprintf("/* %s */\n", componentName))
					mergedStyle.WriteString(style + "\n\n")
				}
			}
		}
	}
	mergedStyle.WriteString("</style>\n")

	return mergedScript.String(), mergedStyle.String()
}

func processComponent(content string, componentName string) (string, error) {
	// 1. 先處理腳本中的變數重命名
	scriptRe := regexp.MustCompile(`(?s)<script>(.*?)</script>`)
	scripts := scriptRe.FindAllStringSubmatch(content, -1)
	var processedContent = content
	var varMap map[string]string

	for _, script := range scripts {
		originalScript := script[1]
		processedScript, newVarMap := renameIdentifiers(originalScript, componentName)
		processedContent = strings.Replace(processedContent, originalScript, processedScript, 1)
		varMap = newVarMap
	}

	// 2. 更新 HTML 中的變數引用
	if varMap != nil {
		for oldName, newName := range varMap {
			// 更新變數引用 {count} -> {App_count}
			processedContent = regexp.MustCompile(`{`+regexp.QuoteMeta(oldName)+`}`).ReplaceAllString(
				processedContent, "{"+newName+"}")
			// 更新事件處理器 on:click={increment} -> on:click={App_increment}
			processedContent = regexp.MustCompile(`on:click={`+regexp.QuoteMeta(oldName)+`}`).ReplaceAllString(
				processedContent, "on:click={"+newName+"}")
		}
	}

	// 3. 處理 CSS 作用域
	styleRe := regexp.MustCompile(`(?s)<style>(.*?)</style>`)
	processedContent = styleRe.ReplaceAllStringFunc(processedContent, func(match string) string {
		cssContent := styleRe.FindStringSubmatch(match)[1]
		// 為每個選擇器加上組件類名
		cssContent = regexp.MustCompile(`([^}]*){`).ReplaceAllStringFunc(cssContent, func(selector string) string {
			selector = strings.TrimSpace(selector[:len(selector)-1])
			if selector != "" && !strings.HasPrefix(selector, "@") {
				selectors := strings.Split(selector, ",")
				for i, s := range selectors {
					s = strings.TrimSpace(s)
					selectors[i] = fmt.Sprintf(".%s %s", componentName, s)
				}
				selector = strings.Join(selectors, ", ")
			}
			return selector + " {"
		})
		return fmt.Sprintf("<style>\n%s\n</style>", cssContent)
	})

	// 4. 為 HTML 元素添加組件類名
	htmlRe := regexp.MustCompile(`<(div|span|section|article|header|footer|main|nav|aside|p|h[1-6]|ul|ol|li|table|tr|td|th)([^>]*?)>`)
	processedContent = htmlRe.ReplaceAllStringFunc(processedContent, func(match string) string {
		if strings.Contains(match, `class=`) {
			return regexp.MustCompile(`class="([^"]*)"`).ReplaceAllString(
				match, fmt.Sprintf(`class="$1 %s"`, componentName))
		}
		tagEnd := strings.Index(match, ">")
		return match[:tagEnd] + fmt.Sprintf(` class="%s"`, componentName) + match[tagEnd:]
	})

	return processedContent, nil
}

func AutoParse() (string, error) {
	fmt.Println("\n=== 步驟 1: 讀取並處理各組件 ===")
	files, err := os.ReadDir("./src")
	if err != nil {
		return "", err
	}

	// 1. 讀取並處理每個組件
	processedComponents := make(map[string]string)
	var allScripts []string
	var allStyles []string

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		content, err := os.ReadFile("./src/" + file.Name())
		if err != nil {
			return "", err
		}
		componentName := strings.TrimSuffix(file.Name(), ".svelte")
		fmt.Printf("\n處理組件 %s:\n", componentName)

		// 提取並處理腳本
		scriptRe := regexp.MustCompile(`(?s)<script>(.*?)</script>`)
		scripts := scriptRe.FindAllStringSubmatch(string(content), -1)
		var processedContent = string(content)

		for _, script := range scripts {
			originalScript := script[1]
			processedScript, varMap := renameIdentifiers(originalScript, componentName)
			fmt.Printf("原始腳本:\n%s\n", originalScript)
			fmt.Printf("處理後腳本:\n%s\n", processedScript)

			// 更新 HTML 中的變數引用
			processedContent = strings.Replace(processedContent, originalScript, "", 1)
			for oldName, newName := range varMap {
				processedContent = regexp.MustCompile(`{`+regexp.QuoteMeta(oldName)+`}`).ReplaceAllString(
					processedContent, "{"+newName+"}")
				processedContent = regexp.MustCompile(`on:click={`+regexp.QuoteMeta(oldName)+`}`).ReplaceAllString(
					processedContent, "on:click={"+newName+"}")
			}

			allScripts = append(allScripts, processedScript)
		}

		// 提取並處理樣式
		styleRe := regexp.MustCompile(`(?s)<style>(.*?)</style>`)
		styles := styleRe.FindAllStringSubmatch(processedContent, -1)
		for _, style := range styles {
			originalStyle := style[1]
			fmt.Printf("原始樣式:\n%s\n", originalStyle)

			processedStyle := regexp.MustCompile(`([^}]*){`).ReplaceAllStringFunc(originalStyle, func(selector string) string {
				selector = strings.TrimSpace(selector[:len(selector)-1])
				if selector != "" && !strings.HasPrefix(selector, "@") {
					selectors := strings.Split(selector, ",")
					for i, s := range selectors {
						s = strings.TrimSpace(s)
						selectors[i] = fmt.Sprintf(".%s %s", componentName, s)
					}
					selector = strings.Join(selectors, ", ")
				}
				return selector + " {"
			})
			fmt.Printf("處理後樣式:\n%s\n", processedStyle)
			allStyles = append(allStyles, processedStyle)

			// 移除原始樣式標籤
			processedContent = strings.Replace(processedContent, style[0], "", 1)
		}

		// 為 HTML 元素添加組件類名
		processedContent = addComponentClass(processedContent, componentName)

		processedComponents[componentName] = processedContent
		fmt.Printf("處理後的組件內容:\n%s\n", processedContent)
	}

	fmt.Println("\n=== 步驟 2: 合併模板 ===")
	// 2. 使用處理後的內容創建模板
	tmpl := template.New("root")
	for name, content := range processedComponents {
		_, err = tmpl.New(name).Parse(content)
		if err != nil {
			return "", err
		}
	}

	// 3. 渲染主模板
	var buf bytes.Buffer
	err = tmpl.ExecuteTemplate(&buf, "App", nil)
	if err != nil {
		return "", err
	}
	mergedContent := buf.String()

	// 4. 移除模板標記
	mergedContent = regexp.MustCompile(`{{define\s+"[^"]+"}}`).ReplaceAllString(mergedContent, "")
	mergedContent = regexp.MustCompile(`{{end}}`).ReplaceAllString(mergedContent, "")

	// 5. 移除空的 script 和 style 標籤
	mergedContent = regexp.MustCompile(`<script>\s*</script>`).ReplaceAllString(mergedContent, "")
	mergedContent = regexp.MustCompile(`<style>\s*</style>`).ReplaceAllString(mergedContent, "")

	// 6. 清理空行
	mergedContent = regexp.MustCompile(`\n\s*\n`).ReplaceAllString(mergedContent, "\n")
	mergedContent = strings.TrimSpace(mergedContent)

	fmt.Println("\n=== 步驟 3: 組合最終內容 ===")
	// 7. 組合最終內容
	finalContent := "<script>\n" + strings.Join(allScripts, "\n\n") + "\n</script>\n" +
		"<style>\n" + strings.Join(allStyles, "\n") + "\n</style>\n" + mergedContent

	fmt.Printf("最終內容:\n%s\n", finalContent)

	// 8. 編譯 Svelte
	res, err := ParseSvelte(finalContent)
	if err != nil {
		return "", err
	}
	fmt.Println("編譯結果:", res)
	return res, nil
}
