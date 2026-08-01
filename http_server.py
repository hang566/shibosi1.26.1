import http.server
import socketserver
import os
import mimetypes

CHARSET_MAP = {
    '.html': 'utf-8',
    '.htm': 'utf-8',
    '.css': 'utf-8',
    '.js': 'utf-8',
    '.json': 'utf-8',
    '.svg': 'utf-8',
    '.xml': 'utf-8',
    '.txt': 'utf-8',
    '.md': 'utf-8',
}


class UTF8Handler(http.server.SimpleHTTPRequestHandler):
    def send_head(self):
        path = self.translate_path(self.path)

        # 如果是目录，自动尝试 index.html
        if os.path.isdir(path):
            for index in ['index.html', 'index.htm']:
                index_path = os.path.join(path, index)
                if os.path.isfile(index_path):
                    path = index_path
                    self.path = self.path.rstrip('/') + '/' + index
                    break

        ctype = self.guess_type(path)
        ext = os.path.splitext(path)[1].lower()
        if ext in CHARSET_MAP and 'charset' not in ctype:
            ctype = ctype + '; charset=' + CHARSET_MAP[ext]
        try:
            f = open(path, 'rb')
        except OSError:
            self.send_error(404, "File not found")
            return None
        try:
            self.send_response(200)
            self.send_header("Content-Type", ctype)
            fs = os.fstat(f.fileno())
            self.send_header("Content-Length", str(fs[6]))
            self.send_header("Last-Modified", self.date_time_string(fs.st_mtime))
            self.send_header("Cache-Control", "no-cache")
            self.end_headers()
            return f
        except Exception:
            f.close()
            raise

    def guess_type(self, path):
        base = super().guess_type(path)
        return base


if __name__ == '__main__':
    import sys
    port = int(sys.argv[1]) if len(sys.argv) > 1 else 8080
    host = sys.argv[2] if len(sys.argv) > 2 else '127.0.0.1'
    socketserver.TCPServer.allow_reuse_address = True
    with socketserver.TCPServer((host, port), UTF8Handler) as httpd:
        print(f"Serving on http://{host}:{port}")
        httpd.serve_forever()
