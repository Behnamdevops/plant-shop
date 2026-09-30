-- Original care guides; never overwrite owner articles or product data.
INSERT INTO articles(title,slug,excerpt,content,status,published_at,seo_title,seo_description) SELECT 'علت زرد شدن برگ گیاهان؛ پیش از خرید کود چه بررسی کنیم؟','yellow-leaves','زردی برگ همیشه به معنی کمبود کود نیست. بررسی شرایط گیاه، نخستین قدم انتخاب محصول است.','## از تشخیص شروع کنید
زردی برگ می‌تواند با آب زیاد یا کم و شرایط محیط ارتباط داشته باشد. خاک، زهکش و سابقهٔ آبیاری را بررسی کنید؛ خرید کود بدون شناخت علت، راه‌حل مشخصی نیست.
## اطلاعات مفید برای بررسی
نام گیاه، عکس برگ و خاک، محل قرارگیری و زمان آخرین آبیاری را ثبت کنید. وضعیت ریشه و برچسب محصولات مصرف‌شده را هم در نظر بگیرید.
## انتخاب محصول بعد از بررسی
اگر بستر نیاز به اصلاح دارد، [خاک و بستر کشت](/category/substrates) را با مشخصات گیاه مقایسه کنید. برای تغذیه، ابتدا [راهنمای انتخاب کود](/blog/fertilizer-guide) را بخوانید. محصولی صرفاً به خاطر زردی برگ توصیه نمی‌شود.
[منبع آموزشی: University of Maryland Extension](https://www.extension.umd.edu/resource/watering-indoor-plants)
','published',NOW(),'علت زرد شدن برگ گیاهان؛ پیش از خرید کود چه بررسی کنیم؟','زردی برگ همیشه به معنی کمبود کود نیست. بررسی شرایط گیاه، نخستین قدم انتخاب محصول است.' WHERE NOT EXISTS(SELECT 1 FROM article_slug_aliases WHERE slug='yellow-leaves') ON CONFLICT(slug) DO NOTHING;
INSERT INTO articles(title,slug,excerpt,content,status,published_at,seo_title,seo_description) SELECT 'رشد گیاه متوقف شده؟ راهنمای بررسی شرایط و تغذیه','slow-growth','پیش از تقویت گیاه، وضعیت محیط، خاک و نشانه‌های آن را بررسی کنید.','## فقط کود را مقصر ندانید
رشد کم همراه زردی برگ می‌تواند با کمبود عناصر ارتباط داشته باشد، اما نشانه‌ها به‌تنهایی علت را ثابت نمی‌کنند. سابقهٔ نگهداری و وضعیت گیاه را بررسی کنید.
## انتخاب با اطلاعات محصول
نوع گیاه، ترکیبات و روش مصرف درج‌شده روی کود را مقایسه کنید. مصرف بیشتر از دستور سازنده می‌تواند مشکل ایجاد کند؛ مقدار و فاصلهٔ مصرف برای همهٔ محصولات یکسان نیست.
[کود و تقویت‌کننده‌ها](/category/fertilizers) را پس از بررسی نیاز گیاه ببینید. برای تنظیم برنامهٔ مصرف، [راهنمای کوددهی](/blog/feeding-schedule) را بخوانید.
[منبع آموزشی: University of Maryland Extension](https://www.extension.umd.edu/resources/yard-garden/indoor-plants/indoor-plant-problems-nonliving)
','published',NOW(),'رشد گیاه متوقف شده؟ راهنمای بررسی شرایط و تغذیه','پیش از تقویت گیاه، وضعیت محیط، خاک و نشانه‌های آن را بررسی کنید.' WHERE NOT EXISTS(SELECT 1 FROM article_slug_aliases WHERE slug='slow-growth') ON CONFLICT(slug) DO NOTHING;
INSERT INTO articles(title,slug,excerpt,content,status,published_at,seo_title,seo_description) SELECT 'راهنمای انتخاب خاک و بستر مناسب گلدان','choosing-substrate','ترکیبات و زهکشی بستر را پیش از خرید با نیاز گیاه مقایسه کنید.','## بستر متناسب با گیاه
بستر تازه می‌تواند به زهکشی و محیط ریشه کمک کند. انتخاب آن باید با نیاز گیاه و شرایط گلدان هماهنگ باشد؛ یک ترکیب برای همهٔ گیاهان مناسب نیست.
## هنگام خرید چه ببینیم؟
ترکیبات، حجم بسته، کاربرد اعلام‌شده و روش استفاده را بخوانید. دربارهٔ خاک آماده، پرلیت یا کوکوپیت، اطلاعات سازنده و نیاز گیاه را با هم در نظر بگیرید؛ نسبت ترکیب را حدس نزنید.
[خاک و بستر کشت](/category/substrates) و [ابزار نگهداری](/category/tools) را با مشخصات ثبت‌شده مقایسه کنید. محصولات مرتبط با این راهنما پس از اتصال توسط مدیر در پایین صفحه نمایش داده می‌شوند.
[منبع آموزشی: UMN Extension](https://extension.umn.edu/garden-and-home/yard-and-garden/gardening-in-minnesota/houseplants/spring-houseplant-care)
','published',NOW(),'راهنمای انتخاب خاک و بستر مناسب گلدان','ترکیبات و زهکشی بستر را پیش از خرید با نیاز گیاه مقایسه کنید.' WHERE NOT EXISTS(SELECT 1 FROM article_slug_aliases WHERE slug='choosing-substrate') ON CONFLICT(slug) DO NOTHING;
INSERT INTO articles(title,slug,excerpt,content,status,published_at,seo_title,seo_description) SELECT 'راهنمای بررسی آفت و انتخاب محصول محافظتی','plant-protection','ابتدا مسئله را شناسایی کنید؛ روش مصرف و هشدار سازنده را بخوانید.','## ابتدا نشانه را بررسی کنید
پیش از انتخاب محصول، برگ‌ها و شرایط گیاه را بررسی و عکس روشن تهیه کنید. علائم مشابه می‌توانند علت‌های متفاوت داشته باشند. محصول محافظتی را بر اساس تشخیص مسئله و کاربرد روی برچسب انتخاب کنید.
## پیش از مصرف
نام آفت یا بیماری، کاربرد مجاز، روش مصرف و هشدارهای سازنده را بررسی کنید. محصولات مختلف را بدون دستور معتبر با هم ترکیب نکنید. راهنمای عمومی جای دستور محصول یا بررسی متخصص را نمی‌گیرد.
[محصولات مراقبت و محافظت](/category/protection) همراه اطلاعات مصرف عرضه می‌شوند. برای بررسی شرایط خاص گیاه، [با پشتیبانی تماس بگیرید](/contact).
[منبع آموزشی: University of Maryland Extension](https://www.extension.umd.edu/resource/diagnose-indoor-plant-problems)
','published',NOW(),'راهنمای بررسی آفت و انتخاب محصول محافظتی','ابتدا مسئله را شناسایی کنید؛ روش مصرف و هشدار سازنده را بخوانید.' WHERE NOT EXISTS(SELECT 1 FROM article_slug_aliases WHERE slug='plant-protection') ON CONFLICT(slug) DO NOTHING;
INSERT INTO articles(title,slug,excerpt,content,status,published_at,seo_title,seo_description) SELECT 'بهترین کود برای گیاهان آپارتمانی چیست؟','fertilizer-guide','کود مناسب با نیاز گیاه و مشخصات محصول انتخاب می‌شود، نه یک نام ثابت برای همه.','## یک کود برای همه نیست
کود دارای برچسب مصرف گیاهان آپارتمانی را با نیاز گیاه مقایسه کنید. ترکیبات، شکل محصول و دستور رقیق‌سازی اهمیت دارند؛ مقدار مصرف را از برچسب همان محصول بخوانید.
## پیش از خرید
برند، وزن یا حجم، مناسب‌بودن و هشدار را بررسی کنید. [کود و تقویت‌کننده‌ها](/category/fertilizers) با مشخصات ثبت‌شده قابل مقایسه‌اند.
[منبع آموزشی: University of Maryland Extension](https://www.extension.umd.edu/resource/fertilizer-indoor-plants)
','published',NOW(),'بهترین کود برای گیاهان آپارتمانی چیست؟','کود مناسب با نیاز گیاه و مشخصات محصول انتخاب می‌شود، نه یک نام ثابت برای همه.' WHERE NOT EXISTS(SELECT 1 FROM article_slug_aliases WHERE slug='fertilizer-guide') ON CONFLICT(slug) DO NOTHING;
INSERT INTO articles(title,slug,excerpt,content,status,published_at,seo_title,seo_description) SELECT 'برنامهٔ کوددهی گیاهان آپارتمانی؛ چطور ثبت کنیم؟','feeding-schedule','برنامه را مطابق دستور محصول و وضعیت گیاه تنظیم و سابقهٔ مصرف را ثبت کنید.','## برنامه را برای محصول خود تنظیم کنید
یک فاصلهٔ ثابت برای همهٔ کودها وجود ندارد. مقدار و زمان مصرف را از دستور سازنده بگیرید و وضعیت گیاه را در نظر بگیرید.
## سابقهٔ مصرف
نام محصول، تاریخ مصرف، مقدار طبق برچسب و نتیجهٔ مشاهده‌شده را ثبت کنید. قبل از تغییر برنامه، [مشخصات کود](/category/fertilizers) و راهنمای مصرف را دوباره بخوانید.
[منبع آموزشی: University of Maryland Extension](https://www.extension.umd.edu/resource/fertilizer-indoor-plants)
','published',NOW(),'برنامهٔ کوددهی گیاهان آپارتمانی؛ چطور ثبت کنیم؟','برنامه را مطابق دستور محصول و وضعیت گیاه تنظیم و سابقهٔ مصرف را ثبت کنید.' WHERE NOT EXISTS(SELECT 1 FROM article_slug_aliases WHERE slug='feeding-schedule') ON CONFLICT(slug) DO NOTHING;
