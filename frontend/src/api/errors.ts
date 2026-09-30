// ApiError carries the HTTP status code alongside the message so callers can
// branch on specific statuses (401/403/404/409) instead of parsing message
// text. Existing api modules that already throw plain Error are left as-is
// to avoid a broad rewrite; new admin endpoints use this.
export class ApiError extends Error {
  status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export async function readErrorMessage(response: Response): Promise<string> {
  const text = (await response.text()).trim()
  const messages: Record<string, string> = {
    'invalid credentials': 'ایمیل یا رمز عبور درست نیست.',
    unauthorized: 'برای ادامه وارد حساب شوید.',
    forbidden: 'اجازه انجام این کار را ندارید.',
    'insufficient stock': 'موجودی محصول کافی نیست.',
    'only verified buyers can review':
      'ثبت نظر برای خریدارانی فعال است که سفارش پرداخت‌شده را تحویل گرفته‌اند.',
    'password recovery is unavailable; contact support':
      'بازیابی رمز فعلاً فعال نیست. با پشتیبانی تماس بگیرید.',
    'password reset link is invalid or expired':
      'لینک بازیابی معتبر نیست یا منقضی شده است.',
  }
  if (messages[text]) return messages[text]
  if (/[\u0600-\u06ff]/.test(text)) return text
  if (response.status === 429)
    return 'درخواست‌ها زیاد است. کمی بعد دوباره تلاش کنید.'
  if (response.status >= 500)
    return 'سرویس موقتاً در دسترس نیست. دوباره تلاش کنید.'
  if (response.status === 413) return 'حجم فایل بیش از حد مجاز است.'
  if (response.status === 404) return 'اطلاعات موردنظر پیدا نشد.'
  if (response.status === 409)
    return 'اطلاعات تغییر کرده است. صفحه را تازه کنید.'
  return 'اطلاعات واردشده را بررسی کنید و دوباره تلاش کنید.'
}

export async function throwApiError(response: Response): Promise<never> {
  throw new ApiError(response.status, await readErrorMessage(response))
}
