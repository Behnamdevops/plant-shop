import { Link } from "react-router-dom";
import { storeConfig } from "../config";
import Icon from "./Icon";
export default function Footer() {
  return (
    <footer className="site-footer">
      <div className="site-footer__container">
        <div className="site-footer__brand">
          <Link className="site-footer__brand-link" to="/">
            <Icon name="leaf" size={32} />
            {storeConfig.name}
          </Link>
          <p>
            محصول و آموزش برای مراقبت بهتر از گیاه.
            <br />
            با شناخت بیشتر انتخاب کنید و بهتر مراقبت کنید.
          </p>
          <Link className="footer-contact" to="/contact">
            ارتباط با فروشگاه <Icon name="arrow" size={17} />
          </Link>
        </div>
        <div className="site-footer__links">
          {[
            {
              title: "فروشگاه",
              links: [
                ["/shop", "همهٔ محصولات"],
                ["/category/fertilizers", "کود و تقویت‌کننده‌ها"],
                ["/category/substrates", "خاک و بستر کشت"],
                ["/wishlist", "علاقه‌مندی‌ها"],
              ],
            },
            {
              title: "حساب و سفارش",
              links: [
                ["/account", "حساب کاربری"],
                ["/orders", "پیگیری سفارش"],
                ["/shipping", "روش و هزینهٔ ارسال"],
                ["/returns", "درخواست مرجوعی"],
              ],
            },
            {
              title: "دنیای گیاهان",
              links: [
                ["/blog", "آموزش و راهنما"],
                ["/about", "دربارهٔ ما"],
                ["/faq", "پرسش‌های متداول"],
                ["/contact", "تماس با ما"],
              ],
            },
          ].map((group) => (
            <div className="site-footer__column" key={group.title}>
              <h3>{group.title}</h3>
              <ul>
                {group.links.map(([to, label]) => (
                  <li key={to}>
                    <Link to={to}>{label}</Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      </div>
      <div className="site-footer__bottom">
        <p>
          © {new Date().getFullYear()} {storeConfig.name} · تمامی حقوق محفوظ
          است.
        </p>
        <div>
          <Link to="/privacy">حریم خصوصی</Link>
          <Link to="/terms">شرایط استفاده</Link>
        </div>
      </div>
    </footer>
  );
}
