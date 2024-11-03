package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/HazelnutParadise/sveltigo/parse"
)

const port = ":3000"

func main() {
	// 創建 dist 目錄
	if err := os.MkdirAll("dist", 0755); err != nil {
		log.Fatal(err)
	}

	// 編譯 Svelte 組件
	result, err := parse.AutoParse()
	if err != nil {
		log.Fatal(err)
	}

	// 將編譯結果寫入文件
	err = os.WriteFile("dist/bundle.js", []byte(result), 0644)
	if err != nil {
		log.Fatal(err)
	}

	// 創建 index.html
	indexHTML := `
<!DOCTYPE html>
<html lang="zh-TW">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Svelte App</title>
</head>
<body>
    <div id="app"></div>
    <script src="/dist/bundle.js"></script>
    <script>
        new App({
            target: document.getElementById('app')
        });
    </script>
</body>
</html>`

	err = os.WriteFile("dist/index.html", []byte(indexHTML), 0644)
	if err != nil {
		log.Fatal(err)
	}

	// 設置路由
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "dist/index.html")
			return
		}
		http.FileServer(http.Dir(".")).ServeHTTP(w, r)
	})

	fmt.Printf("Server running at http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
