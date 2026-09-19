import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { storeConfig } from '../config'
import SEO from '../components/SEO'

type FAQItem = {
  question: string
  answer: string
  category: 'order' | 'payment' | 'shipping' | 'care' | 'cancel' | 'coupon'
}

const faqItems: FAQItem[] = [
  // Order / سفارش
  {
    category: 'order',
    question: 'چگونه سفارش دهم؟',
    answer: 'برای سفارش، محصول مورد نظر را انتخاب کرده و دکمه «افزودن به سبد خرید» را بزنید. سپس به صفحه سبد خرید مراجعه کرده و اطلاعات خود را وارد کنید.'
  },
  {
    category: 'order',
    question: 'آیا نیاز به ثبت‌نام دارم؟',
    answer: 'ثبت‌نام اجباری نیست، اما برای ردیابی سفارش و دریافت پشتیبانی بهتر، توصیه می‌کنیم حساب کاربری بسازید.'
  },
  {
    category: 'order',
    question: 'چگونه وضعیت سفارش را بررسی کنم؟',
    answer: 'پس از ورود به حساب کاربری خود، می‌توانید در بخش «سفارش‌های من» وضعیت تمامی سفارش‌های خود را مشاهده کنید.'
  },

  // Payment / پرداخت
  {
    category: 'payment',
    question: 'روش‌های پرداخت چیست؟',
    answer: 'ما از درگاه زرین‌پال برای پرداخت‌های آنلاین استفاده می‌کنیم. تمامی کارت‌های بانکی متصل به شبا و کارت‌های اعتباری شناور در شبکه شتاب قابل قبول هستند.'
  },
  {
    category: 'payment',
    question: 'آیا پرداخت آنلاین امن است؟',
    answer: 'بله، تمامی تراکن‌های پرداخت از طریق درگاه زرین‌پال که یکی از معتبرترین درگاه‌های پرداخت ایران است، انجام می‌شود.'
  },
  {
    category: 'payment',
    question: 'می‌توانم پس از سفارش، پرداخت کنم؟',
    answer: 'خیر، پرداخت باید پیش از تکمیل سفارش انجام شود.'
  },

  // Shipping / ارسال
  {
    category: 'shipping',
    question: 'ارسال محصولات چگونه انجام می‌شود؟',
    answer: 'محصولات پس از تایید پرداخت، در اولین اولویت برای ارسال آماده می‌شوند. زمان تحویل بستگی به منطقه و شرایط حمل دارد.'
  },
  {
    category: 'shipping',
    question: 'هزینه ارسال چقدر است؟',
    answer: 'هزینه ارسال بر اساس منطقه و وزن سفارش محاسبه می‌شود. در صفحه سبد خرید قبل از پرداخت، هزینه ارسال به شما نمایش داده می‌شود.'
  },
  {
    category: 'shipping',
    question: 'آیا ارسال رایگان وجود دارد؟',
    answer: 'بله، برای سفارش‌های بالای مبلغ مشخص، هزینه ارسال رایگان ارائه می‌شود.'
  },

  // Care / نگهداری
  {
    category: 'care',
    question: 'آیا راهنمای نگهداری محصولات ارائه می‌شود؟',
    answer: 'بله، در توضیحات هر محصول، نیازهای خاص آن گیاه (آبیاری، نور، دما و ...) ذکر شده است.'
  },
  {
    category: 'care',
    question: 'اگر گیاه بعد از خرید بیمار شد، چه کنم؟',
    answer: 'اگر گیاه پس از دریافت به دلیل حمل و نقل یا عوامل دیگر آسیب ببیند، با پشتیبانی تماس بگیرید. ما تلاش می‌کنیم بهترین راه حل را ارائه دهیم.'
  },
  {
    category: 'care',
    question: 'چگونه از آسیب گیاهان در حمل و نقل جلوگیری شود؟',
    answer: 'گیاهان به صورت حرفه‌ای بسته‌بندی می‌شوند تا در حین حمل و نقل آسیب نبینند. با این حال، پس از دریافت گیاه، آن را به محیط جدید آشکار کنید.'
  },

  // Cancel / لغو
  {
    category: 'cancel',
    question: 'آیا می‌توانم سفارش را لغو کنم؟',
    answer: 'بله، تا زمانی که سفارش به مرحله ارسال نرسیده، امکان لغو آن وجود دارد. لغو سفارش از طریق حساب کاربری یا تماس با پشتیبانی انجام می‌شود.'
  },
  {
    category: 'cancel',
    question: 'پس از لغو سفارش، چه می‌شود؟',
    answer: 'مبلغ سفارش لغو شده به حساب شما برمی‌گردد. زمان بازگشت مبلغ بستگی به روش پرداخت و بانک شما دارد.'
  },

  // Coupon / کد تخفیف
  {
    category: 'coupon',
    question: 'کد تخفیف چگونه استفاده شود؟',
    answer: 'در صفحه سبد خرید، کد تخفیف را وارد کرده و دکمه «اعمال کد» را بزنید. مبلغ تخفیف بر اساس قوانین آن کد محاسبه می‌شود.'
  },
  {
    category: 'coupon',
    question: 'کد تخفیف محدودیت دارد؟',
    answer: 'بله، هر کد تخفیف ممکن است محدودیت‌هایی در مبلغ سفارش، تاریخ انقضا یا محصولات قابل استفاده داشته باشد. لطفاً توضیحات هر کد را دقت کنید.'
  },
  {
    category: 'coupon',
    question: 'آیا چند کد تخفیف قابل استفاده است؟',
    answer: 'خیر، در هر سفارش تنها می‌توان از یک کد ��خفیف استفاده کرد.'
  }
]

const categoryDisplayNames: Record<FAQItem['category'] | 'all', string> = {
  order: 'سفارش',
  payment: 'پرداخت',
  shipping: 'ارسال',
  care: 'نگهداری',
  cancel: 'لغو سفارش',
  coupon: 'کد تخفیف',
  all: 'همه'
}

export default function FAQPage() {
  const pageTitle = 'سوالات متداول'
  const pageDescription = 'سوالات متداول در مورد سفارش، پرداخت، ارسال و نگهداری گیاهان'

  const [activeCategory, setActiveCategory] = useState<FAQItem['category'] | 'all'>('all')
  const [openIndices, setOpenIndices] = useState<number[]>([])

  useEffect(() => {
    document.title = `${pageTitle} - ${storeConfig.name}`
    const descMeta = document.querySelector('meta[name="description"]')
    if (descMeta) {
      descMeta.setAttribute('content', pageDescription)
    }
  }, [])

  const filteredItems = activeCategory === 'all'
    ? faqItems
    : faqItems.filter(item => item.category === activeCategory)

  const toggleFAQ = (index: number) => {
    if (openIndices.includes(index)) {
      setOpenIndices(openIndices.filter(i => i !== index))
    } else {
      setOpenIndices([...openIndices, index])
    }
  }

  const categories: Array<FAQItem['category'] | 'all'> = ['all', 'order', 'payment', 'shipping', 'care', 'cancel', 'coupon']

  return (
    <div className="faq-page">
      <SEO title={pageTitle} description={pageDescription} />
      <div className="faq-header">
        <h1>سوالات متداول</h1>
        <p>
          در اینجا به پاسخ به سوالات رایج مشتریان پرداخته‌ایم. اگر پاسخ سوال خود را پیدا نکردید،
          از طریق <Link to="/contact">تماس با ما</Link> به ما اطلاع دهید.
        </p>
      </div>

      <div className="faq-categories">
        {categories.map(cat => (
          <button
            key={cat}
            className={`faq-category-btn ${activeCategory === cat ? 'active' : ''}`}
            onClick={() => setActiveCategory(cat)}
          >
            {categoryDisplayNames[cat]}
          </button>
        ))}
      </div>

      <div className="faq-list">
        {filteredItems.length === 0 ? (
          <div className="faq-empty">
            <p>سوالی یافت نشد.</p>
          </div>
        ) : (
          filteredItems.map((item, index) => {
            const faqIndex = faqItems.findIndex(i => i === item)
            const isOpen = openIndices.includes(faqIndex)
            return (
              <div className={`faq-item ${isOpen ? 'open' : ''}`} key={index}>
                <button
                  className="faq-question"
                  onClick={() => toggleFAQ(faqIndex)}
                  aria-expanded={isOpen}
                >
                  <span>{item.question}</span>
                  <span className="faq-toggle" aria-hidden="true">
                    {isOpen ? '−' : '+'}
                  </span>
                </button>
                {isOpen && (
                  <div className="faq-answer">
                    <p>{item.answer}</p>
                  </div>
                )}
              </div>
            )
          })
        )}
      </div>

      <div className="faq-cta">
        <p>سوال دیگری دارید؟</p>
        <Link to="/contact" className="btn btn-primary">
          تماس با ما
        </Link>
      </div>
    </div>
  )
}
