import { test, expect } from '@playwright/test'
test('public HTML is complete without JavaScript, with real 404 and sitemap', async ({
  request,
  browser,
}) => {
  const result = await request.get('/api/v1/products?in_stock=true')
  expect(result.ok()).toBeTruthy()
  const product = (await result.json()).items[0]
  expect(
    product,
    'Seed at least one product in the disposable E2E database',
  ).toBeTruthy()
  const context = await browser.newContext({ javaScriptEnabled: false })
  const page = await context.newPage()
  await page.goto(`/products/${product.slug}`)
  await expect(
    page.getByRole('heading', { name: product.name, exact: true }),
  ).toBeVisible()
  await expect(page.locator('script[type="application/ld+json"]')).toHaveCount(
    1,
  )
  const missing = await request.get('/products/does-not-exist-e2e')
  expect(missing.status()).toBe(404)
  const sitemap = await request.get('/sitemap.xml')
  expect(await sitemap.text()).toContain(product.slug)
  await context.close()
})
test('guest cart and wishlist survive refresh and merge after registration', async ({
  page,
  request,
}) => {
  const product = (
    await (await request.get('/api/v1/products?in_stock=true')).json()
  ).items[0]
  const errors: string[] = []
  page.on('pageerror', (e) => errors.push(e.message))
  await page.goto(`/products/${product.slug}`)
  await page
    .getByRole('button', { name: 'افزودن به سبد خرید', exact: true })
    .click()
  await expect(page.getByRole('status')).toContainText('به سبد خرید اضافه شد')
  await page
    .getByRole('button', { name: /افزودن به علاقه/ })
    .first()
    .click()
  await page.reload()
  await expect(page.getByRole('button', { name: /حذف از علاقه/ })).toBeVisible()
  await page.goto('/cart')
  await expect(
    page.getByRole('link', { name: product.name, exact: true }),
  ).toBeVisible()
  await page.reload()
  await expect(
    page.getByRole('link', { name: product.name, exact: true }),
  ).toBeVisible()
  await page.goto('/register?returnTo=%2Fcart')
  await page.getByLabel('نام', { exact: true }).fill('خریدار آزمایش')
  await page
    .getByLabel('ایمیل', { exact: true })
    .fill(`buyer-${Date.now()}@example.com`)
  await page.getByLabel('رمز عبور', { exact: true }).fill('Password123!')
  await page.getByRole('button', { name: 'ثبت‌نام', exact: true }).click()
  await expect(page).toHaveURL(/\/cart$/)
  await expect(
    page.getByRole('link', { name: product.name, exact: true }),
  ).toBeVisible()
  const cart = await page.request.get('/api/v1/cart')
  expect((await cart.json()).items[0].quantity).toBe(1)
  const saved = await page.request.get('/api/v1/wishlist')
  expect(
    (await saved.json()).some((p: { id: number }) => p.id === product.id),
  ).toBeTruthy()
  const review = await page.request.post(
    `/api/v1/products/${product.id}/reviews`,
    { data: { rating: 5, comment: 'خرید انجام نشده است' } },
  )
  expect(review.status()).toBe(403)
  expect(errors).toEqual([])
})
test('mobile menu is usable and page does not overflow', async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 })
  await page.goto('/shop')
  await page.getByRole('button', { name: /منو/ }).click()
  await expect(
    page.getByRole('link', { name: 'علاقه‌مندی‌ها', exact: true }).first(),
  ).toBeVisible()
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBeTruthy()
  await page.screenshot({
    path: 'test-results/mobile-shop.png',
    fullPage: true,
  })
})
test('private routes cannot expose admin data and checkout has no guest spinner', async ({
  page,
  request,
}) => {
  await page.goto('/checkout')
  await expect(page.getByRole('link', { name: /ورود/ }).last()).toBeVisible()
  const response = await request.get('/api/v1/admin/reviews')
  expect([401, 403]).toContain(response.status())
  const privateHTML = await request.get('/admin/reviews')
  expect(privateHTML.headers()['x-robots-tag']).toContain('noindex')
})
