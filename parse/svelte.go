package parse

import (
	"fmt"
	"io"
	"net/http"

	"rogchap.com/v8go"
)

func ParseSvelte(componentCode string) (string, error) {
	ctx := v8go.NewContext()

	// 添加 structuredClone 的 polyfill
	polyfillScript := `
		if (typeof structuredClone === 'undefined') {
			function structuredClone(obj) {
				return JSON.parse(JSON.stringify(obj));
			}
		}
	`
	_, err := ctx.RunScript(polyfillScript, "polyfill.js")
	if err != nil {
		fmt.Println("Error adding polyfill:", err)
		return "", err
	}

	// 從 CDN 下載 Svelte 編譯器代碼
	response, err := http.Get("https://unpkg.com/svelte@5.1.9/compiler/index.js")
	if err != nil {
		fmt.Println("Failed to download Svelte compiler:", err)
		return "", err
	}
	defer response.Body.Close()

	// 讀取編譯器代碼內容
	compilerCode, err := io.ReadAll(response.Body)
	if err != nil {
		fmt.Println("Failed to read compiler code:", err)
		return "", err
	}

	// 在 V8 執行上下文中執行 Svelte 編譯器代碼
	_, err = ctx.RunScript(string(compilerCode), "compiler.js")
	if err != nil {
		fmt.Println("Error executing Svelte compiler:", err)
		return "", err
	}

	// 執行編譯 Svelte 組件的腳本
	compileScript := fmt.Sprintf(`
        var compiled = svelte.compile(%q, { generate: "ssr" });
        compiled.js.code;
    `, componentCode)

	result, err := ctx.RunScript(compileScript, "compile.js")
	if err != nil {
		fmt.Println("Error compiling Svelte component:", err)
		return "", err
	}

	// 輸出編譯結果
	return string(result.String()), nil
}
