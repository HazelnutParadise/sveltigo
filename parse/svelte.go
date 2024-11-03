package parse

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
)

func downloadCompiler() error {
	// 創建臨時目錄
	if err := os.MkdirAll("temp", 0755); err != nil {
		return err
	}

	// 下載編譯器
	resp, err := http.Get("https://unpkg.com/svelte@5.1.9/compiler/index.js")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// 保存到臨時文件
	out, err := os.Create("temp/svelte-compiler.js")
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func ParseSvelte(content string) (string, error) {
	// 確保編譯器存在
	if _, err := os.Stat("temp/svelte-compiler.js"); os.IsNotExist(err) {
		if err := downloadCompiler(); err != nil {
			return "", fmt.Errorf("error downloading compiler: %v", err)
		}
	}

	// 創建臨時的 Svelte 編譯配置
	config := map[string]interface{}{
		"generate":   "server",
		"css":        "injected",
		"dev":        false,
		"hydratable": false,
	}

	configJSON, err := json.Marshal(config)
	if err != nil {
		return "", err
	}

	// 準備 Node.js 腳本
	script := fmt.Sprintf(`
		const svelte = require('./temp/svelte-compiler.js');
		const component = %q;
		const config = %s;

		try {
			const result = svelte.compile(component, config);
			console.log(result.js.code);
		} catch (error) {
			console.error('Error compiling Svelte component:', error.message);
			process.exit(1);
		}
	`, content, configJSON)

	// 執行 Node.js 腳本
	cmd := exec.Command("node", "-e", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("compilation error: %v\n%s", err, stderr.String())
	}

	return stdout.String(), nil
}
