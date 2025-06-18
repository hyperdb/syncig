package main

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Config struct {
	SRC_DIR      string   `json:"SRC_DIR"`
	DIST_DIR     string   `json:"DIST_DIR"`
	EXCLUDED_EXT []string `json:"EXCLUDED_EXT"`
}

func loadConfig(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var cfg Config
	if err := json.NewDecoder(f).Decode(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// 指定拡張子が除外対象か判定
func isExcluded(ext string, excludes []string) bool {
	ext = strings.ToLower(ext)
	for _, e := range excludes {
		if strings.ToLower(e) == ext {
			return true
		}
	}
	return false
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

func readLastCopiedFile(distDir string) (string, error) {
	lastFile := filepath.Join(distDir, "last_copied.txt")
	b, err := os.ReadFile(lastFile)
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func writeLastCopiedFile(distDir, filename string) error {
	lastFile := filepath.Join(distDir, "last_copied.txt")
	return os.WriteFile(lastFile, []byte(filename), 0644)
}

func copyFile(srcFile, distFile string) error {
	srcF, err := os.Open(srcFile)
	if err != nil {
		return err
	}
	defer srcF.Close()
	dstF, err := os.Create(distFile)
	if err != nil {
		return err
	}
	defer dstF.Close()
	_, err = io.Copy(dstF, srcF)
	return err
}

func syncDir(srcRoot, distRoot string, excludedExt []string) error {
	// サブディレクトリごとに処理
	return filepath.WalkDir(srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() || path == srcRoot {
			return nil
		}
		rel, _ := filepath.Rel(srcRoot, path)
		distDir := filepath.Join(distRoot, rel)

		// サブディレクトリ内のファイル一覧を取得
		entries, err := os.ReadDir(path)
		if err != nil {
			return err
		}
		var files []string
		for _, entry := range entries {
			if entry.Type().IsRegular() {
				ext := filepath.Ext(entry.Name())
				if isExcluded(ext, excludedExt) {
					continue
				}
				// ファイルサイズ0判定
				info, err := entry.Info()
				if err != nil {
					continue
				}
				if info.Size() == 0 {
					continue
				}
				files = append(files, entry.Name())
			}
		}
		if len(files) == 0 {
			return nil
		}
		sort.Strings(files)
		lastCopied, err := readLastCopiedFile(distDir)
		if err != nil {
			return err
		}
		toCopy := []string{}
		for _, f := range files {
			if lastCopied == "" || f > lastCopied {
				toCopy = append(toCopy, f)
			}
		}
		if len(toCopy) == 0 {
			return nil
		}
		// コピー処理
		if err := ensureDir(distDir); err != nil {
			return err
		}
		for _, f := range toCopy {
			srcFile := filepath.Join(path, f)
			distFile := filepath.Join(distDir, f)
			if err := copyFile(srcFile, distFile); err != nil {
				return err
			}
			fmt.Printf("Copied: %s -> %s\n", srcFile, distFile)
		}
		// 最後にコピーしたファイル名を記録（最大値）
		if err := writeLastCopiedFile(distDir, toCopy[len(toCopy)-1]); err != nil {
			return err
		}
		return nil
	})
}

func main() {
	cfg, err := loadConfig("config.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "loadConfig error: %v\n", err)
		os.Exit(1)
	}
	srcDir := strings.TrimRight(cfg.SRC_DIR, string(os.PathSeparator))
	distDir := strings.TrimRight(cfg.DIST_DIR, string(os.PathSeparator))
	if err := syncDir(srcDir, distDir, cfg.EXCLUDED_EXT); err != nil {
		fmt.Fprintf(os.Stderr, "syncDir error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Sync completed.")
}
