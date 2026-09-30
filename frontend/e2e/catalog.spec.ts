import { test, expect } from "@playwright/test";

test("catalog quick add survives refresh; unavailable products stay disabled", async ({
  page,
  request,
}) => {
  const products = (await (await request.get("/api/v1/products")).json()).items;
  const available = products.find((p: { stock: number }) => p.stock > 0);
  await page.goto("/shop");
  const card = page
    .locator(".product-card")
    .filter({
      has: page.getByRole("heading", { name: available.name, exact: true }),
    });
  await card
    .getByRole("button", {
      name: `افزودن ${available.name} به سبد خرید`,
      exact: true,
    })
    .click();
  await expect(card.getByRole("status")).toContainText("به سبد اضافه شد");
  await page.goto("/cart");
  await page.reload();
  await expect(
    page.getByRole("link", { name: available.name, exact: true }),
  ).toBeVisible();
  const unavailable = products.find((p: { stock: number }) => p.stock === 0);
  if (unavailable) {
    await page.goto("/shop");
    await expect(
      page.getByRole("button", {
        name: `افزودن ${unavailable.name} به سبد خرید`,
        exact: true,
      }),
    ).toBeDisabled();
  }
});

test("toman filters convert to rial, clearing cannot restore a stale draft", async ({
  page,
}) => {
  await page.goto("/shop?kind=fertilizer");
  await expect(page.getByRole("button", { name: "حذف فیلترها" })).toBeVisible();
  await page.getByLabel("حداقل قیمت (تومان)").fill("300000");
  await expect(page).toHaveURL(/min_price=3000000/);
  await page.getByRole("button", { name: "حذف فیلترها" }).click();
  await expect(page).toHaveURL(/\/shop$/);
  await page.waitForTimeout(650);
  await expect(page).toHaveURL(/\/shop$/);
  await expect(page.getByLabel("حداقل قیمت (تومان)")).toHaveValue("");
  await page
    .getByLabel("جست‌وجو در فروشگاه", { exact: true })
    .fill("کود کامل آزمایشی");
  await page.getByLabel("جست‌وجو در فروشگاه", { exact: true }).press("Enter");
  await expect(page.getByLabel("جستجوی محصول", { exact: true })).toHaveValue(
    "کود کامل آزمایشی",
  );
  await expect(page.locator(".product-card")).toHaveCount(1);
});

test("home layout, botanical asset and image placeholders remain usable on mobile", async ({
  page,
  request,
}) => {
  expect((await request.get("/botanical-scene.svg")).status()).toBe(200);
  await page.setViewportSize({ width: 390, height: 844 });
  const errors: string[] = [];
  page.on("pageerror", (error) => errors.push(error.message));
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toBeVisible();
  await page
    .getByRole("link", { name: /کود و تقویت‌کننده‌ها/ })
    .first()
    .click();
  await expect(page).toHaveURL(/category\/fertilizers/);
  await expect(page.locator(".product-card").first()).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy();
  expect(errors).toEqual([]);
});
