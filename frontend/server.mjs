import http from "node:http";
import { readFile } from "node:fs/promises";
import { resolve, extname, sep } from "node:path";
import { render } from "./dist/server/entry-server.js";
const root = resolve("dist"),
  template = await readFile(resolve(root, "index.html"), "utf8");
const api = (process.env.API_ORIGIN || "http://127.0.0.1:8080").replace(
  /\/$/,
  "",
);
const origin = new URL(process.env.PUBLIC_BASE_URL || "http://localhost:4173")
  .origin;
const storeName = process.env.STORE_NAME || "گیاکو";
const escape = (value) =>
  String(value || "").replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );
const titles = {
  "/": "فروشگاه تخصصی کود، خاک و محصولات مراقبت از گیاه",
  "/shop": "خرید کود، خاک و محصولات مراقبت از گیاه",
  "/blog": "آموزش نگهداری و تغذیهٔ گیاه",
  "/about": "درباره ما",
  "/contact": "تماس با ما",
  "/faq": "سوالات متداول",
  "/shipping": "ارسال و حمل",
  "/returns": "بازگرداندن کالا",
  "/privacy": "حریم خصوصی",
  "/terms": "شرایط استفاده",
};
const mime = {
  ".js": "text/javascript",
  ".css": "text/css",
  ".svg": "image/svg+xml",
  ".ttf": "font/ttf",
  ".png": "image/png",
  ".jpg": "image/jpeg",
  ".webp": "image/webp",
  ".ico": "image/x-icon",
};
async function get(path) {
  const response = await fetch(api + path, {
    signal: AbortSignal.timeout(5000),
  });
  if (!response.ok) {
    const error = new Error("API unavailable");
    error.status = response.status;
    throw error;
  }
  return response.json();
}
const server = http.createServer(async (req, res) => {
  try {
    const url = new URL(req.url, "http://internal"),
      path = url.pathname,
      data = { path: path + url.search, origin };
    if (path === "/healthz") {
      res.writeHead(200, { "Content-Type": "application/json" });
      res.end('{"status":"ok"}');
      return;
    }
    if (
      /^\/(assets|fonts)\//.test(path) ||
      [
        "/favicon.svg",
        "/og-default.svg",
        "/og-default.png",
        "/botanical-scene.svg",
        "/care-products.svg",
      ].includes(path)
    ) {
      const file = resolve(root, '.' + decodeURIComponent(path)),
        prefix =
          resolve(
            root,
            path.startsWith('/assets/')
              ? 'assets'
              : path.startsWith('/fonts/')
                ? 'fonts'
                : '.',
          ) + sep
      if (!file.startsWith(prefix)) {
        res.writeHead(404);
        res.end();
        return;
      }
      const body = await readFile(file);
      res.writeHead(200, {
        "Content-Type": mime[extname(file)] || "application/octet-stream",
        "Cache-Control": "public, max-age=31536000, immutable",
      });
      res.end(req.method === "HEAD" ? undefined : body);
      return;
    }
    if (
      path.startsWith("/api/") ||
      path.startsWith("/uploads/") ||
      path === "/sitemap.xml" ||
      path === "/robots.txt"
    ) {
      const headers = { host: req.headers.host };
      for (const key of ["cookie", "origin", "sec-fetch-site", "content-type"])
        if (req.headers[key]) headers[key] = req.headers[key];
      let body;
      if (!["GET", "HEAD"].includes(req.method)) {
        let bytes = 0;
        const chunks = [];
        for await (const chunk of req) {
          bytes += chunk.length;
          if (bytes > 6 * 1024 * 1024) {
            res.writeHead(413);
            res.end();
            return;
          }
          chunks.push(chunk);
        }
        body = Buffer.concat(chunks);
      }
      const response = await fetch(api + path + url.search, {
        method: req.method,
        headers,
        body,
        redirect: "manual",
        signal: AbortSignal.timeout(30000),
      });
      const out = {
        "Content-Type": response.headers.get("content-type") || "text/plain",
      };
      for (const key of ["cache-control", "location", "x-request-id"])
        if (response.headers.get(key)) out[key] = response.headers.get(key);
      const cookies = response.headers.getSetCookie();
      if (cookies.length) out["set-cookie"] = cookies;
      res.writeHead(response.status, out);
      res.end(
        req.method === "HEAD"
          ? undefined
          : Buffer.from(await response.arrayBuffer()),
      );
      return;
    }
    if (!["GET", "HEAD"].includes(req.method)) {
      res.writeHead(405);
      res.end();
      return;
    }
    if (
      /^\/(admin|account|cart|checkout|orders|payment|login|register|wishlist|forgot-password|reset-password)(\/|$)/.test(
        path,
      )
    ) {
      res.writeHead(200, {
        "Content-Type": "text/html; charset=utf-8",
        "Cache-Control": "no-store",
        "X-Robots-Tag": "noindex, nofollow",
      });
      res.end(req.method === "HEAD" ? undefined : template);
      return;
    }
    if (path === "/articles" || path.startsWith("/articles/")) {
      res.writeHead(301, {
        Location: path.replace("/articles", "/blog") + url.search,
      });
      res.end();
      return;
    }
    let status = 200,
      title = titles[path] || "صفحه پیدا نشد",
      description = `${title} در ${storeName}؛ محصولات مراقبت، تغذیه و آموزش گیاه با مشخصات و روش مصرف.`,
      canonical = path,
      image = "/og-default.png";
    data.categories = await get("/api/v1/categories");
    if (path === "/")
      [data.products, data.articles] = await Promise.all([
        get("/api/v1/products?sort=bestselling&page_size=6"),
        get("/api/v1/articles?page_size=4"),
      ]);
    else if (path === "/shop") {
      const params = new URLSearchParams(url.search);
      params.set("page_size", "20");
      data.products = await get("/api/v1/products?" + params);
    } else if (path === "/blog") {
      const params = new URLSearchParams(url.search);
      params.set("page_size", "12");
      [data.articles, data.articleCategories] = await Promise.all([
        get("/api/v1/articles?" + params),
        get("/api/v1/article-categories"),
      ]);
    } else if (/^\/category\/[^/]+\/?$/.test(path)) {
      const category = data.categories.find(
        (c) => c.slug === decodeURIComponent(path.split("/")[2]),
      );
      if (!category) {
        status = 404;
        title = "دسته‌بندی یافت نشد";
      } else {
        title = "خرید " + category.name;
        description = `${title}؛ مشخصات، برند، روش مصرف و قیمت در ${storeName}.`;
        canonical = "/category/" + encodeURIComponent(category.slug);
        const params = new URLSearchParams(url.search);
        params.set("category", String(category.id));
        params.set("page_size", "20");
        data.products = await get("/api/v1/products?" + params);
      }
    } else if (/^\/products\/[^/]+\/?$/.test(path)) {
      try {
        data.product = await get(
          "/api/v1/products/" +
            encodeURIComponent(decodeURIComponent(path.split("/")[2])),
        );
      } catch (error) {
        if (error.status !== 404) throw error;
        status = 404;
      }
      if (data.product) {
        title = data.product.name;
        description = data.product.description;
        canonical = "/products/" + encodeURIComponent(data.product.slug);
        image = data.product.image_url || image;
        data.relatedProducts = await get(
          "/api/v1/products/" +
            encodeURIComponent(data.product.slug) +
            "/related",
        );
      }
    } else if (/^\/blog\/[^/]+\/?$/.test(path)) {
      try {
        data.article = await get(
          "/api/v1/articles/" +
            encodeURIComponent(decodeURIComponent(path.split("/")[2])),
        );
      } catch (error) {
        if (error.status !== 404) throw error;
        status = 404;
      }
      if (data.article) {
        title = data.article.seo_title || data.article.title;
        description = data.article.seo_description || data.article.excerpt;
        canonical = "/blog/" + encodeURIComponent(data.article.slug);
        image = data.article.cover_image_url || image;
        data.relatedProducts = (
          await get(
            "/api/v1/products?guide=" +
              encodeURIComponent(data.article.slug) +
              "&in_stock=true&page_size=8",
          )
        ).items;
      }
    } else if (path === "/shipping")
      data.shipping = await get("/api/v1/shipping");
    else if (!titles[path]) status = 404;
    if (status === 200 && decodeURI(path) !== decodeURI(canonical)) {
      res.writeHead(301, { Location: canonical });
      res.end();
      return;
    }
    const html = await render(data.path, data);
    const metadata = `<title>${escape(title)} - ${escape(storeName)}</title><meta name="description" content="${escape(String(description || "").slice(0, 300))}"><link rel="canonical" href="${escape(origin + canonical)}"><meta name="robots" content="${status === 404 ? "noindex, follow" : "index, follow"}"><meta property="og:title" content="${escape(title)}"><meta property="og:description" content="${escape(String(description || "").slice(0, 300))}"><meta property="og:image" content="${escape(new URL(image, origin).href)}"><meta property="og:url" content="${escape(origin + canonical)}"><meta property="og:type" content="${data.article ? "article" : "website"}"><meta name="twitter:card" content="summary_large_image">`;
    const seed = JSON.stringify(data)
      .replace(/</g, "\\u003c")
      .replace(/\u2028/g, "\\u2028")
      .replace(/\u2029/g, "\\u2029");
    const document = template
      .replace(/<title>.*?<\/title>/s, metadata)
      .replace(
        '<div id="root"></div>',
        `<div id="root">${html}</div><script id="public-data" type="application/json">${seed}</script>`,
      );
    res.writeHead(status, {
      "Content-Type": "text/html; charset=utf-8",
      "Cache-Control": "no-cache",
      "X-Content-Type-Options": "nosniff",
    });
    res.end(req.method === "HEAD" ? undefined : document);
  } catch (error) {
    if (error.code === "ENOENT") {
      res.writeHead(404);
      res.end();
      return;
    }
    const status = error.status === 400 ? 400 : 503;
    console.error("storefront request failed");
    res.writeHead(status, {
      "Content-Type": "text/plain; charset=utf-8",
      "Retry-After": "30",
    });
    res.end(
      status === 400
        ? "پارامترهای جست‌وجو معتبر نیستند."
        : "فروشگاه موقتاً در دسترس نیست. دوباره تلاش کنید.",
    );
  }
});
server.listen(Number(process.env.PORT || 4173), "0.0.0.0");
for (const signal of ["SIGTERM", "SIGINT"])
  process.on(signal, () => server.close(() => process.exit(0)));
