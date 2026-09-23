const http = require("http");
const fs = require("fs");
const path = require("path");

const socketPath = process.env.SOCKET_PATH;
if (!socketPath) {
  console.error("SOCKET_PATH required");
  process.exit(1);
}
try { fs.unlinkSync(socketPath); } catch (_) {}

const configured = process.env.STATIC_ROOT || "dist";
const absRoot = path.isAbsolute(configured) ? configured : path.join(process.cwd(), configured);
if (!fs.existsSync(absRoot)) {
  console.error("static root missing:", absRoot);
  process.exit(1);
}
const root = fs.realpathSync(absRoot);
const mime = {
  ".html": "text/html; charset=utf-8",
  ".js": "application/javascript; charset=utf-8",
  ".mjs": "application/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".json": "application/json",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".jpeg": "image/jpeg",
  ".webp": "image/webp",
  ".gif": "image/gif",
  ".ico": "image/x-icon",
  ".woff": "font/woff",
  ".woff2": "font/woff2",
  ".ttf": "font/ttf",
  ".map": "application/json",
  ".txt": "text/plain; charset=utf-8",
  ".xml": "application/xml",
};

function send(res, status, body, type) {
  res.writeHead(status, {
    "Content-Type": type || "text/plain; charset=utf-8",
    "Cache-Control": status === 200 ? "public, max-age=60" : "no-store",
  });
  res.end(body);
}

const server = http.createServer((req, res) => {
  try {
    const urlPath = decodeURIComponent((req.url || "/").split("?")[0]);
    let rel = urlPath === "/" ? "/index.html" : urlPath;
    let file = path.normalize(path.join(root, rel));
    if (!file.startsWith(root + path.sep) && file !== root) {
      return send(res, 403, "Forbidden");
    }
    let st;
    try { st = fs.statSync(file); } catch (_) { st = null; }
    if (!st || st.isDirectory()) {
      file = path.join(root, "index.html");
    }
    const ext = path.extname(file).toLowerCase();
    send(res, 200, fs.readFileSync(file), mime[ext] || "application/octet-stream");
  } catch (e) {
    send(res, 500, "Internal error");
  }
});

server.listen(socketPath, () => {
  try { fs.chmodSync(socketPath, 0o666); } catch (_) {}
  console.log("listening on", socketPath);
});
