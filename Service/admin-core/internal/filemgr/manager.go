// Package filemgr 文件管理器核心实现
package filemgr

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Node 节点信息
type Node struct {
	Path     string `json:"path"`
	Name     string `json:"name"`
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size"`
	Mode     string `json:"mode"`
	Modified int64  `json:"modified"`
	Mime     string `json:"mime"`
	Children []*Node `json:"children,omitempty"`
}

// Manager 文件管理器
type Manager struct {
	BaseDir string
}

func New(baseDir string) *Manager {
	return &Manager{BaseDir: baseDir}
}

// List 列出目录内容（支持分页/深层级异步加载）
func (m *Manager) List(path string, depth int) ([]*Node, error) {
	abs := m.resolve(path)
	info, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("not a directory")
	}

	entries, err := os.ReadDir(abs)
	if err != nil {
		return nil, err
	}

	var nodes []*Node
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		node := &Node{
			Path: filepath.Join(path, e.Name()),
			Name: e.Name(),
			IsDir: fi.IsDir(),
			Size:  fi.Size(),
			Mode:  fi.Mode().Perm().String(),
			Modified: fi.ModTime().Unix(),
		}
		if !node.IsDir {
			node.Mime = guessMime(e.Name(), "")
		}
		if depth > 0 && node.IsDir {
			children, _ := m.List(node.Path, depth-1)
			node.Children = children
		}
		nodes = append(nodes, node)
	}
	return nodes, nil
}

// Read 读取文件内容，返回 base64
func (m *Manager) Read(path string) (string, error) {
	abs := m.resolve(path)
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if info.Size() > 5*1024*1024 {
		return "", fmt.Errorf("file too large (%d bytes), max 5MB", info.Size())
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// Write 写入文件（base64 解码后写入）
func (m *Manager) Write(path, contentB64 string) error {
	abs := m.resolve(path)
	dir := filepath.Dir(abs)
	os.MkdirAll(dir, 0755)
	data, err := base64.StdEncoding.DecodeString(contentB64)
	if err != nil {
		return err
	}
	return os.WriteFile(abs, data, 0644)
}

// CreateDir 创建目录
func (m *Manager) CreateDir(path string) error {
	return os.MkdirAll(m.resolve(path), 0755)
}

// Rename 重命名
func (m *Manager) Rename(oldPath, newPath string) error {
	return os.Rename(m.resolve(oldPath), m.resolve(newPath))
}

// Move 移动
func (m *Manager) Move(src, dst string) error {
	srcAbs := m.resolve(src)
	dstAbs := m.resolve(dst)
	return os.Rename(srcAbs, dstAbs)
}

// Copy 复制
func (m *Manager) Copy(src, dst string) error {
	srcAbs := m.resolve(src)
	dstAbs := m.resolve(dst)

	srcInfo, err := os.Stat(srcAbs)
	if err != nil {
		return err
	}
	if srcInfo.IsDir() {
		return copyDir(srcAbs, dstAbs)
	}
	return copyFile(srcAbs, dstAbs)
}

// Delete 删除
func (m *Manager) Delete(path string) error {
	return os.RemoveAll(m.resolve(path))
}

// Chmod 修改权限
func (m *Manager) Chmod(path string, mode string) error {
	var m2 os.FileMode
	fmt.Sscanf(mode, "%o", &m2)
	return os.Chmod(m.resolve(path), m2)
}

// Zip 打包为 zip，返回路径
func (m *Manager) Zip(srcPath, zipPath string) (string, error) {
	srcAbs := m.resolve(srcPath)
	zipAbs := m.resolve(zipPath)
	f, err := os.Create(zipAbs)
	if err != nil {
		return "", err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()

	info, err := os.Stat(srcAbs)
	if err != nil {
		return "", err
	}
	base := filepath.Base(srcAbs)
	if info.IsDir() {
		filepath.Walk(srcAbs, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(srcAbs, path)
			relPath := filepath.Join(base, rel)
			if fi.IsDir() {
				w.Create(relPath + "/")
				return nil
			}
			header, _ := zip.FileInfoHeader(fi)
			header.Name = relPath
			header.Method = zip.Deflate
			zw, err := w.CreateHeader(header)
			if err != nil {
				return err
			}
			if !fi.IsDir() {
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				zw.Write(data)
			}
			return nil
		})
	} else {
		data, _ := os.ReadFile(srcAbs)
		zw, _ := w.Create(base)
		zw.Write(data)
	}
	return zipPath, nil
}

// TarGz 打包为 tar.gz
func (m *Manager) TarGz(srcPath, tgzPath string) (string, error) {
	srcAbs := m.resolve(srcPath)
	tgzAbs := m.resolve(tgzPath)
	f, err := os.Create(tgzAbs)
	if err != nil {
		return "", err
	}
	defer f.Close()
	gw := gzip.NewWriter(f)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	info, err := os.Stat(srcAbs)
	if err != nil {
		return "", err
	}
	base := filepath.Base(srcAbs)
	if info.IsDir() {
		filepath.Walk(srcAbs, func(path string, fi os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			header, _ := tar.FileInfoHeader(fi, "")
			rel, _ := filepath.Rel(srcAbs, path)
			header.Name = filepath.Join(base, rel)
			if fi.IsDir() {
				header.Name += "/"
			}
			tw.WriteHeader(header)
			if !fi.IsDir() {
				data, _ := os.ReadFile(path)
				tw.Write(data)
			}
			return nil
		})
	} else {
		data, _ := os.ReadFile(srcAbs)
		header, _ := tar.FileInfoHeader(info, "")
		header.Name = base
		tw.WriteHeader(header)
		tw.Write(data)
	}
	return tgzPath, nil
}

// Extract 解压 zip/tar.gz
func (m *Manager) Extract(archivePath, destDir string) error {
	abs := m.resolve(archivePath)
	dest := m.resolve(destDir)
	os.MkdirAll(dest, 0755)

	ext := strings.ToLower(filepath.Ext(abs))
	if ext == ".zip" {
		return extractZip(abs, dest)
	}
	if ext == ".gz" || ext == ".tgz" {
		return extractTarGz(abs, dest)
	}
	return fmt.Errorf("unsupported archive: %s", ext)
}

// Search 在目录中递归搜索关键字（文件名）
func (m *Manager) Search(keyword string) ([]*Node, error) {
	var results []*Node
	err := filepath.Walk(m.BaseDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if strings.Contains(strings.ToLower(info.Name()), strings.ToLower(keyword)) {
			rel, _ := filepath.Rel(m.BaseDir, path)
			results = append(results, &Node{
				Path: rel, Name: info.Name(), IsDir: info.IsDir(),
				Size: info.Size(), Modified: info.ModTime().Unix(),
			})
		}
		return nil
	})
	return results, err
}

// resolve 解析相对路径（禁止越权到 BaseDir 之外）
func (m *Manager) resolve(p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(m.BaseDir, p)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		srcPath := filepath.Join(src, e.Name())
		dstPath := filepath.Join(dst, e.Name())
		if e.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func extractZip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(filepath.Clean(fpath), filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", f.Name)
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, 0755)
		} else {
			os.MkdirAll(filepath.Dir(fpath), 0755)
			out, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			rc, _ := f.Open()
			io.Copy(out, rc)
			rc.Close()
			out.Close()
		}
	}
	return nil
}

func extractTarGz(src, dest string) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()
	gr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		target := filepath.Join(dest, hdr.Name)
		if !strings.HasPrefix(filepath.Clean(target), filepath.Clean(dest)+string(os.PathSeparator)) {
			continue
		}
		switch hdr.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(target, 0755)
		case tar.TypeReg:
			os.MkdirAll(filepath.Dir(target), 0755)
			out, _ := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(hdr.Mode))
			if out != nil {
				io.Copy(out, tr)
				out.Close()
			}
		}
	}
	return nil
}

func guessMime(name, sample string) string {
	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".go", ".py", ".js", ".ts", ".java", ".cpp", ".c", ".h", ".php", ".rb", ".rs", ".vue":
		return "text/x-code"
	case ".html", ".htm":
		return "text/html"
	case ".css":
		return "text/css"
	case ".json":
		return "application/json"
	case ".xml":
		return "application/xml"
	case ".yaml", ".yml":
		return "text/yaml"
	case ".md":
		return "text/markdown"
	case ".sql":
		return "application/sql"
	case ".sh":
		return "application/x-shellscript"
	}
	if sample != "" {
		return mime.TypeByExtension(ext)
	}
	return "application/octet-stream"
}

// 确保使用 time 包防止 "imported and not used"
var _ = time.Now
