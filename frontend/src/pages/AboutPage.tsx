import { useEffect } from 'react'
import { storeConfig } from '../config'
import SectionHeader from '../components/SectionHeader'
import Hero from '../components/Hero'
import SEO from '../components/SEO'

export default function AboutPage() {
  const pageTitle = 'درباره ما'
  const pageDescription = 'درباره ' + storeConfig.name + '، فروشگاه گیاهات و آموزش نگهداری گیاهان'
  const aboutTitle = `چرا ${storeConfig.name}؟`
  const aboutOfferingTitle = 'ما چه ارائه می‌دهیم؟'
  const joinTitle = 'به ما بپیوندید'

  useEffect(() => {
    document.title = `${pageTitle} - ${storeConfig.name}`
    const descMeta = document.querySelector('meta[name="description"]')
    if (descMeta) {
      descMeta.setAttribute('content', pageDescription)
    }
  }, [])

  return (
    <div className="about-page">
      <SEO title={pageTitle} description={pageDescription} />
      <Hero
        title="درباره ما"
        subtitle="فروشگاه آنلاین گیاهات با هدف آموزش و ارائه بهترین محصولات"
        primaryText="مشاهده محصولات"
        primaryLink="/shop"
        secondaryText="مشاهده مقالات"
        secondaryLink="/articles"
      />

      <div className="page-content">
        <div className="content-section">
          <SectionHeader
            title={aboutTitle}
            align="center"
          />
          <div className="about-content">
            <p>
              {storeConfig.name} با هدف ارتقای سبک زندگی و آموزش نگهداری صحیح از گیاهان شروع به کار کرد.
              ما باور داریم که گیاهان نه تنها محیط زیست را زیباتر می‌کنند، بلکه سلامت روان و بدن شما را نیز بهبود می‌بخشند.
            </p>
            <p>
              در {storeConfig.name} محصولاتی را ارائه می‌دهیم که بر اساس معیارهای کیفیت و اهمیت برای مشتریان ما انتخاب شده‌اند.
              ما به آموزش نگهداری صحیح گیاهان نیز توجه ویژه‌ای داریم تا بتوانید بهترین نتیجه را از خرید خود بگیرید.
            </p>

            <div className="about-benefits">
              <div className="about-benefit">
                <h3>گیاهان سالم</h3>
                <p>فقط گیاهات با کیفیت و سالم برای شما ارسال می‌شود</p>
              </div>
              <div className="about-benefit">
                <h3>راهنمای کامل</h3>
                <p>آموزش‌های جامع برای نگهداری صحیح گیاهان</p>
              </div>
              <div className="about-benefit">
                <h3>خرید امن</h3>
                <p>پرداخت آنلاین امن و ارتباط مستقیم با مشتریان</p>
              </div>
            </div>
          </div>
        </div>

        <div className="content-section">
          <SectionHeader
            title={aboutOfferingTitle}
            align="center"
          />
          <div className="about-offering">
            <div className="offering-card">
              <h3>گیاهات مختلف</h3>
              <p>گیاهات داخل خانه، بیرون خانه، گل‌ها و گیاهات اطراف خانه</p>
            </div>
            <div className="offering-card">
              <h3>محصولات مکمل</h3>
              <p>کود، آبیاری خودکار، گلدان و دیگر ابزارهای مورد نیاز</p>
            </div>
            <div className="offering-card">
              <h3>آموزش و آگاهی</h3>
              <p>مقالات و آموزش‌های جامع در مورد نگهداری گیاهان</p>
            </div>
          </div>
        </div>

        <div className="content-section">
          <SectionHeader
            title={joinTitle}
            align="center"
            linkTo="/shop"
            linkText="مشاهده محصولات"
          />
          <p className="about-cta">
            ما با امید به همراهی شما در این سفر سبز و سالم شروع به کار کردیم.
            امیدواریم بتوانیم با ارائه محصولات با کیفیت و آموزش‌های مفید، به بهبود کیفیت زندگی شما کمک کنیم.
          </p>
        </div>
      </div>
    </div>
  )
}
