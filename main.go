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
<!doctype html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Vite + Svelte + TS</title>
  </head>
  <body>
    <div id="app"></div>
    <script type="module" src="/src/main.js"></script>
  </body>
</html>
`

	err = os.WriteFile("dist/index.html", []byte(indexHTML), 0644)
	if err != nil {
		log.Fatal(err)
	}

	// 設置路由
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			http.ServeFile(w, r, "dist/index.html")
			return
		}
		// 為 JS 文件設置正確的 MIME 類型
		if r.URL.Path == "/dist/bundle.js" {
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		}
		http.FileServer(http.Dir(".")).ServeHTTP(w, r)
	})

	http.HandleFunc("/src/main.js", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "dist/main.js")
	})

	fmt.Printf("Server running at http://localhost%s\n", port)
	log.Fatal(http.ListenAndServe(port, nil))
}
