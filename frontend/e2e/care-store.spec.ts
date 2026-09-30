import { test, expect } from "@playwright/test";

test("care category and guide HTML are indexed, aliases redirect and metadata is present", async ({
  request,
}) => {
  const category = await request.get("/category/fertilizers");
  expect(category.status()).toBe(200);
  expect(await category.text()).toContain("خرید کود و تقویت‌کننده‌ها");
  expect((await request.get("/category/no-such-category")).status()).toBe(404);
  const guide = await request.get("/blog/fertilizer-guide");
  expect(guide.status()).toBe(200);
  expect(await guide.text()).toContain("بهترین کود");
  const old = await request.get("/articles", { maxRedirects: 0 });
  expect(old.status()).toBe(301);
  expect(old.headers().location).toBe("/blog");
  const sitemap = await request.get("/sitemap.xml");
  expect(await sitemap.text()).toContain("/category/fertilizers");
  expect(await sitemap.text()).toContain("/blog/fertilizer-guide");
});

test("administrator creates and edits a care product with details, the buyer can create an unpaid order", async ({
  page,
  request,
}) => {
  test.skip(
    !process.env.E2E_ADMIN_EMAIL,
    "Set a disposable admin account for this test",
  );
  const slug = `care-ui-${Date.now()}`;
  await page.goto("/login?returnTo=%2Fadmin%2Fproducts%2Fnew");
  await page
    .getByLabel("ایمیل", { exact: true })
    .fill(process.env.E2E_ADMIN_EMAIL!);
  await page.getByLabel("رمز عبور", { exact: true }).fill("CareTest123!");
  await page.getByRole("button", { name: "ورود", exact: true }).click();
  await expect(page.getByRole("heading", { name: "محصول جدید" })).toBeVisible();
  await page
    .getByLabel("نوع محصول", { exact: true })
    .selectOption("fertilizer");
  await page.getByLabel("برند", { exact: true }).fill("برند آزمایشی UI");
  await page.getByLabel("وزن / حجم", { exact: true }).fill("۱ لیتر");
  await page.getByLabel("شکل یا نوع مصرف", { exact: true }).fill("مایع");
  await page.getByLabel("روش مصرف", { exact: true }).fill("مطابق برچسب سازنده");
  await page
    .getByLabel("هشدار مصرف", { exact: true })
    .fill("دور از دسترس کودکان");
  await page
    .getByLabel("Slug مقالهٔ راهنما", { exact: true })
    .fill("fertilizer-guide");
  await page.getByLabel("نام", { exact: true }).fill("کود آزمایشی رابط کاربری");
  await page.getByLabel("Slug", { exact: true }).fill(slug);
  await page
    .getByLabel("توضیحات", { exact: true })
    .fill("محصول موقت برای تست کامل فروشگاه");
  await page.getByLabel("قیمت (تومان)", { exact: true }).fill("298000");
  await page.getByLabel("موجودی", { exact: true }).fill("9");
  await page
    .getByLabel("دسته‌بندی", { exact: true })
    .selectOption({ label: "کود و تقویت‌کننده‌ها" });
  await page.getByRole("button", { name: "ایجاد محصول", exact: true }).click();
  await expect(page).toHaveURL(/\/admin\/products$/);
  let product = await (await request.get(`/api/v1/products/${slug}`)).json();
  expect(product.price).toBe(2980000);
  expect(product.details.kind).toBe("fertilizer");
  expect(product.details.usage).toBe("مطابق برچسب سازنده");
  await page.goto(`/admin/products/${product.id}/edit`);
  await page.getByLabel("برند", { exact: true }).fill("برند ویرایش‌شده");
  await page.getByRole("button", { name: /ذخیره/ }).click();
  await expect(page).toHaveURL(/\/admin\/products$/);
  await page.goto(`/products/${slug}`);
  await expect(
    page.getByText("برند ویرایش‌شده", { exact: true }),
  ).toBeVisible();
  await expect(
    page.getByText("دور از دسترس کودکان", { exact: true }),
  ).toBeVisible();
  await page
    .getByRole("button", { name: "افزودن به سبد خرید", exact: true })
    .click();
  await expect(page.getByRole("status")).toContainText("به سبد خرید اضافه شد");
  // A configured payment service is a prerequisite for this UI. Override its
  // availability only; create the real order, while payment stays unpaid.
  await page.route("**/api/v1/shipping", async (route) => {
    const response = await route.fetch();
    const json = await response.json();
    await route.fulfill({
      response,
      json: { ...json, configured: true, payments_enabled: true },
    });
  });
  await page.goto("/checkout");
  await page.getByLabel("نام گیرنده", { exact: true }).fill("خریدار آزمایشی");
  await page.getByLabel("شماره موبایل", { exact: true }).fill("09120000000");
  await page.locator("#address_line1").fill("نشانی آزمایشی");
  await page.locator("#city").fill("تهران");
  await page.locator("#postal_code").fill("1234567890");
  await page.getByRole("button", { name: /ثبت سفارش/ }).click();
  await expect(page).toHaveURL(/\/orders\/\d+$/);
  const id = page.url().split("/").pop();
  const order = await (await page.request.get(`/api/v1/orders/${id}`)).json();
  expect(order.payment_status).toBe("pending");
  expect(order.items[0].unit_price).toBe(2980000);
  product = await (await request.get(`/api/v1/products/${slug}`)).json();
  expect(product.stock).toBe(8);
  await page.goto("/blog/fertilizer-guide");
  await expect(
    page
      .locator(".guide-products")
      .getByRole("heading", { name: "کود آزمایشی رابط کاربری" }),
  ).toBeVisible();
});

test("education delivery has no shipping fee and cannot be used to avoid physical shipping", async ({
  page,
  request,
}) => {
  const products = (
    await (await request.get("/api/v1/products?kind=education")).json()
  ).items;
  test.skip(
    !products.length,
    "Seed an education product in the disposable database",
  );
  await page.goto("/register");
  await page.getByLabel("نام", { exact: true }).fill("آموزش آزمایش");
  await page
    .getByLabel("ایمیل", { exact: true })
    .fill(`education-${Date.now()}@example.com`);
  await page.getByLabel("رمز عبور", { exact: true }).fill("CareTest123!");
  await page.getByRole("button", { name: "ثبت‌نام", exact: true }).click();
  await expect(page).not.toHaveURL(/\/register/);
  expect(
    (
      await page.request.post("/api/v1/cart/items", {
        data: { product_id: products[0].id, quantity: 1 },
      })
    ).ok(),
  ).toBeTruthy();
  const order = await page.request.post("/api/v1/orders", {
    data: {
      recipient_name: "خریدار آموزش",
      phone: "09120000000",
      shipping_method: "digital",
    },
  });
  expect(order.status()).toBe(201);
  const data = await order.json();
  expect(data.shipping_fee).toBe(0);
  expect(data.shipping_method).toBe("digital");
  const physical = (
    await (
      await request.get("/api/v1/products?kind=fertilizer&in_stock=true")
    ).json()
  ).items[0];
  await page.request.post("/api/v1/cart/items", {
    data: { product_id: physical.id, quantity: 1 },
  });
  const bad = await page.request.post("/api/v1/orders", {
    data: {
      recipient_name: "خریدار آموزش",
      phone: "09120000000",
      shipping_method: "digital",
    },
  });
  expect(bad.status()).toBe(400);
});
